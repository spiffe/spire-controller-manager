/*
Copyright 2021 SPIRE Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package spireentry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"google.golang.org/grpc/codes"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	spirev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	"github.com/spiffe/spire-controller-manager/pkg/k8sapi"
	"github.com/spiffe/spire-controller-manager/pkg/namespace"
	"github.com/spiffe/spire-controller-manager/pkg/reconciler"
	"github.com/spiffe/spire-controller-manager/pkg/spireapi"
)

const (
	// joinTokenSpiffePrefix is the prefix that is the part of the parent SPIFFE ID for join token entries.
	// Ref: https://github.com/spiffe/spire/blob/v1.8.7/pkg/server/api/agent/v1/service.go#L714
	// nolint: gosec // not a credential
	joinTokenSpiffePrefix = "/spire/agent/join_token/"

	// joinTokenSelectorType is the selector type used in the selector for join token entries.
	// Ref: https://github.com/spiffe/spire/blob/v1.8.7/pkg/server/api/agent/v1/service.go#L515
	// nolint: gosec // not a credential
	joinTokenSelectorType = "spiffe_id"
)

type ReconcilerConfig struct {
	TrustDomain          spiffeid.TrustDomain
	ClusterName          string
	ClusterDomain        string
	EntryClient          spireapi.EntryClient
	K8sClient            client.Client
	IgnoreNamespaces     []*regexp.Regexp
	AutoPopulateDNSNames bool
	ClassName            string
	WatchClassless       bool
	ParentIDTemplate     *template.Template
	Reconcile            spirev1alpha1.ReconcileConfig
	EntryIDPrefix        string
	EntryIDPrefixCleanup *string

	// GCInterval how long to sit idle (i.e. untriggered) before doing
	// another reconcile.
	GCInterval time.Duration
}

func Reconciler(config ReconcilerConfig) reconciler.Reconciler {
	r := &entryReconciler{
		config: config,
	}
	return reconciler.New(reconciler.Config{
		Kind:       "entry",
		Reconcile:  r.reconcile,
		GCInterval: config.GCInterval,
	})
}

type entryReconciler struct {
	config ReconcilerConfig

	unsupportedFields        map[spireapi.Field]struct{}
	nextGetUnsupportedFields time.Time
}

func (r *entryReconciler) reconcile(ctx context.Context) {
	log := log.FromContext(ctx)

	if time.Now().After(r.nextGetUnsupportedFields) {
		r.recalculateUnsupportFields(ctx, log)
	}
	unsupportedFields := r.unsupportedFields

	// Load current entries from SPIRE server.
	currentEntries, deleteOnlyEntries, err := r.listEntries(ctx)
	if err != nil {
		log.Error(err, "Failed to list SPIRE entries")
		return
	}

	// Populate the existing state
	state := make(entriesState)
	for _, entry := range currentEntries {
		state.AddCurrent(entry)
	}

	clusterStaticEntries := []*ClusterStaticEntry{}
	//	if r.config.Reconcile.ClusterStaticEntries {
	// Load and add entry state for ClusterStaticEntries
	clusterStaticEntries, err = r.listClusterStaticEntries(ctx)
	if err != nil {
		log.Error(err, "Failed to list ClusterStaticEntries")
		return
	}
	r.addClusterStaticEntryEntriesState(ctx, state, clusterStaticEntries)
	//	}

	clusterSPIFFEIDs := []*ClusterSPIFFEID{}
	if r.config.Reconcile.ClusterSPIFFEIDs {
		// Load and add entry state for ClusterSPIFFEIDs
		clusterSPIFFEIDs, err = r.listClusterSPIFFEIDs(ctx)
		if err != nil {
			log.Error(err, "Failed to list ClusterSPIFFEIDs")
			return
		}
		r.addClusterSPIFFEIDEntriesState(ctx, state, clusterSPIFFEIDs)
	}

	var toDelete []spireapi.Entry
	var toCreate []declaredEntry
	var toUpdate []declaredEntry

	for _, s := range state {
		// Sort declared entries.
		sortDeclaredEntriesByPreference(s.Declared)
		if len(s.Declared) > 0 {
			// Grab the first to set.
			preferredEntry := s.Declared[0]
			preferredEntry.By.IncrementEntriesToSet()

			// Record the remaining as masked.
			for _, otherEntry := range s.Declared[1:] {
				otherEntry.By.IncrementEntriesMasked()
			}

			// Borrow the current entry ID if available, for the update. Then
			// drop the current entry from the list so it isn't added to the
			// "to delete" list.
			if len(s.Current) == 0 {
				if preferredEntry.Entry.ID == "" && r.config.EntryIDPrefix != "" {
					preferredEntry.Entry.ID = fmt.Sprintf("%s%s", r.config.EntryIDPrefix, uuid.New())
				}
				toCreate = append(toCreate, preferredEntry)
			} else {
				preferredEntry.Entry.ID = s.Current[0].ID
				if outdatedFields := getOutdatedEntryFields(preferredEntry.Entry, s.Current[0], unsupportedFields); len(outdatedFields) != 0 {
					// Current field does not match. Nothing to do.
					toUpdate = append(toUpdate, preferredEntry)
				}
				s.Current = s.Current[1:]
			}
		}

		// Any remaining current entries that are not associated with join tokens
		// should be removed as they aren't going to be reused for the entry update.
		toDelete = append(toDelete, filterJoinTokenEntries(s.Current)...)
	}

	toDelete = append(toDelete, deleteOnlyEntries...)
	if len(toDelete) > 0 {
		r.deleteEntries(ctx, toDelete)
	}
	if len(toCreate) > 0 {
		r.createEntries(ctx, toCreate)
	}
	if len(toUpdate) > 0 {
		r.updateEntries(ctx, toUpdate)
	}

	// Update the ClusterStaticEntry statuses
	for _, clusterStaticEntry := range clusterStaticEntries {
		log := log.WithValues(clusterStaticEntryLogKey, objectName(clusterStaticEntry))

		if clusterStaticEntry.Status == clusterStaticEntry.NextStatus {
			continue
		}
		clusterStaticEntry.Status = clusterStaticEntry.NextStatus
		if r.config.K8sClient == nil {
			continue
		}
		if err := r.config.K8sClient.Status().Update(ctx, &clusterStaticEntry.ClusterStaticEntry); err == nil {
			log.Info("Updated status")
		} else {
			log.Error(err, "Failed to update status")
		}
	}

	// Update the ClusterSPIFFEID statuses
	for _, clusterSPIFFEID := range clusterSPIFFEIDs {
		log := log.WithValues(clusterSPIFFEIDLogKey, objectName(clusterSPIFFEID))

		if clusterSPIFFEID.Status == clusterSPIFFEID.NextStatus {
			continue
		}
		clusterSPIFFEID.Status = clusterSPIFFEID.NextStatus
		if err := r.config.K8sClient.Status().Update(ctx, &clusterSPIFFEID.ClusterSPIFFEID); err == nil {
			log.Info("Updated status")
		} else {
			log.Error(err, "Failed to update status")
		}
	}
}

func (r *entryReconciler) reconcileClass(className string) bool {
	return (className == "" && r.config.WatchClassless) || className == r.config.ClassName
}

func (r *entryReconciler) recalculateUnsupportFields(ctx context.Context, log logr.Logger) {
	unsupportedFields, err := r.getUnsupportedFields(ctx)
	if err != nil {
		log.Error(err, "failed to get unsupported fields")
		return
	}

	// Get the list of new fields that are marked as unsupported
	var newUnsupportedFields []string
	for key := range unsupportedFields {
		if _, ok := r.unsupportedFields[key]; !ok {
			newUnsupportedFields = append(newUnsupportedFields, string(key))
		}
	}
	if len(newUnsupportedFields) > 0 {
		log.Info("New unsupported fields in SPIRE server found", "fields", strings.Join(newUnsupportedFields, ","))
	}

	// Get the list of fields that used to be unsupported but now are supported
	var supportedFields []string
	for key := range r.unsupportedFields {
		if _, ok := unsupportedFields[key]; !ok {
			supportedFields = append(supportedFields, string(key))
		}
	}
	if len(supportedFields) > 0 {
		log.Info("Fields previously unsupported are now supported on SPIRE server", "fields", strings.Join(supportedFields, ","))
	}

	r.unsupportedFields = unsupportedFields
	r.nextGetUnsupportedFields = time.Now().Add(10 * time.Minute)
}

func (r *entryReconciler) shouldProcessOrDeleteEntryID(entry spireapi.Entry) (bool, bool) {
	if r.config.EntryIDPrefix == "" {
		return true, false
	}
	if strings.HasPrefix(entry.ID, r.config.EntryIDPrefix) {
		return true, false
	}
	if r.config.EntryIDPrefixCleanup != nil {
		cleanupPrefix := *r.config.EntryIDPrefixCleanup
		if cleanupPrefix == "" {
			return false, !strings.Contains(entry.ID, ".")
		}
		if strings.HasPrefix(entry.ID, cleanupPrefix) {
			return false, true
		}
	}
	return false, false
}

func (r *entryReconciler) listEntries(ctx context.Context) ([]spireapi.Entry, []spireapi.Entry, error) {
	// TODO: cache?
	var deleteOnlyEntries []spireapi.Entry
	var currentEntries []spireapi.Entry
	tmpvals, err := r.config.EntryClient.ListEntries(ctx)
	if err != nil {
		return currentEntries, deleteOnlyEntries, err
	}
	for _, value := range tmpvals {
		proc, del := r.shouldProcessOrDeleteEntryID(value)
		if proc {
			currentEntries = append(currentEntries, value)
		}
		if del {
			deleteOnlyEntries = append(deleteOnlyEntries, value)
		}
	}
	return currentEntries, deleteOnlyEntries, nil
}

func (r *entryReconciler) getUnsupportedFields(ctx context.Context) (map[spireapi.Field]struct{}, error) {
	return r.config.EntryClient.GetUnsupportedFields(ctx, r.config.TrustDomain.Name())
}

func (r *entryReconciler) listClusterStaticEntries(ctx context.Context) ([]*ClusterStaticEntry, error) {
	var clusterStaticEntries []spirev1alpha1.ClusterStaticEntry
	var err error
	if r.config.K8sClient != nil {
		clusterStaticEntries, err = k8sapi.ListClusterStaticEntries(ctx, r.config.K8sClient)
	} else {
		//FIXME scheme and path
		scheme := runtime.NewScheme()
		clusterStaticEntries, err = spirev1alpha1.ListClusterStaticEntries(ctx, scheme, "/etc/spire-controller-manager/manifests")
	}
	if err != nil {
		return nil, err
	}
	out := make([]*ClusterStaticEntry, 0, len(clusterStaticEntries))
	for _, clusterStaticEntry := range clusterStaticEntries {
		if r.reconcileClass(clusterStaticEntry.Spec.ClassName) {
			out = append(out, &ClusterStaticEntry{
				ClusterStaticEntry: clusterStaticEntry,
			})
		}
	}
	return out, nil
}

func (r *entryReconciler) listClusterSPIFFEIDs(ctx context.Context) ([]*ClusterSPIFFEID, error) {
	clusterSPIFFEIDs, err := k8sapi.ListClusterSPIFFEIDs(ctx, r.config.K8sClient)
	if err != nil {
		return nil, err
	}
	out := make([]*ClusterSPIFFEID, 0, len(clusterSPIFFEIDs))
	for _, clusterSPIFFEID := range clusterSPIFFEIDs {
		if r.reconcileClass(clusterSPIFFEID.Spec.ClassName) {
			out = append(out, &ClusterSPIFFEID{
				ClusterSPIFFEID: clusterSPIFFEID,
			})
		}
	}
	return out, nil
}

func (r *entryReconciler) listNamespaces(ctx context.Context, namespaceSelector labels.Selector) ([]corev1.Namespace, error) {
	return k8sapi.ListNamespaces(ctx, r.config.K8sClient, namespaceSelector)
}

func (r *entryReconciler) listNamespacePods(ctx context.Context, namespace string, podSelector labels.Selector) ([]corev1.Pod, error) {
	return k8sapi.ListNamespacePods(ctx, r.config.K8sClient, namespace, podSelector)
}

func (r *entryReconciler) addClusterStaticEntryEntriesState(ctx context.Context, state entriesState, clusterStaticEntries []*ClusterStaticEntry) {
	log := log.FromContext(ctx)
	for _, clusterStaticEntry := range clusterStaticEntries {
		log := log.WithValues(clusterSPIFFEIDLogKey, objectName(clusterStaticEntry))
		entry, err := renderStaticEntry(&clusterStaticEntry.Spec)
		if err != nil {
			log.Error(err, "Failed to render ClusterStaticEntry")
			clusterStaticEntry.NextStatus.Rendered = false
			continue
		}
		clusterStaticEntry.NextStatus.Rendered = true
		state.AddDeclared(*entry, clusterStaticEntry)
	}
}

func (r *entryReconciler) addClusterSPIFFEIDEntriesState(ctx context.Context, state entriesState, clusterSPIFFEIDs []*ClusterSPIFFEID) {
	log := log.FromContext(ctx)
	for _, clusterSPIFFEID := range clusterSPIFFEIDs {
		log := log.WithValues(clusterSPIFFEIDLogKey, objectName(clusterSPIFFEID))

		spec, err := spirev1alpha1.ParseClusterSPIFFEIDSpec(&clusterSPIFFEID.Spec)
		if err != nil {
			// TODO: should this be prevented via admission webhook? should
			// we dump this failure into the status?
			log.Error(err, "Failed to parse ClusterSPIFFEID spec")
			continue
		}

		// List namespaces applicable to the ClusterSPIFFEID
		namespaces, err := r.listNamespaces(ctx, spec.NamespaceSelector)
		if err != nil {
			log.Error(err, "Failed to list namespaces")
			continue
		}

		clusterSPIFFEID.NextStatus.Stats.NamespacesSelected += len(namespaces)

		for i := range namespaces {
			if namespace.IsIgnored(r.config.IgnoreNamespaces, namespaces[i].Name) {
				clusterSPIFFEID.NextStatus.Stats.NamespacesIgnored++
				continue
			}

			log := log.WithValues(namespaceLogKey, objectName(&namespaces[i]))

			pods, err := r.listNamespacePods(ctx, namespaces[i].Name, spec.PodSelector)
			switch {
			case err == nil:
			case apierrors.IsNotFound(err):
				continue
			default:
				log.Error(err, "Failed to list namespace pods")
				continue
			}

			clusterSPIFFEID.NextStatus.Stats.PodsSelected += len(pods)
			for i := range pods {
				log := log.WithValues(podLogKey, objectName(&pods[i]))

				entry, err := r.renderPodEntry(ctx, spec, &pods[i])
				switch {
				case err != nil:
					log.Error(err, "Failed to render entry")
					clusterSPIFFEID.NextStatus.Stats.PodEntryRenderFailures++
				case entry != nil:
					// renderPodEntry will return a nil entry if requisite k8s
					// objects disappeared from underneath.
					state.AddDeclared(*entry, clusterSPIFFEID)
				}
			}
		}
	}
}

func (r *entryReconciler) renderPodEntry(ctx context.Context, spec *spirev1alpha1.ParsedClusterSPIFFEIDSpec, pod *corev1.Pod) (*spireapi.Entry, error) {
	// TODO: should we be caching this? probably not since it grabs from the
	// controller client, which is cached already.
	node := new(corev1.Node)
	if err := r.config.K8sClient.Get(ctx, types.NamespacedName{Name: pod.Spec.NodeName}, node); err != nil {
		return nil, client.IgnoreNotFound(err)
	}
	endpointsList := &corev1.EndpointsList{}
	if spec.AutoPopulateDNSNames {
		if err := r.config.K8sClient.List(ctx, endpointsList, client.InNamespace(pod.Namespace), client.MatchingFields{reconciler.EndpointUID: string(pod.UID)}); err != nil && !apierrors.IsNotFound(err) {
			return nil, err
		}
	}
	return renderPodEntry(spec, node, pod, endpointsList, r.config.TrustDomain, r.config.ClusterName, r.config.ClusterDomain, r.config.ParentIDTemplate)
}

func (r *entryReconciler) createEntries(ctx context.Context, declaredEntries []declaredEntry) {
	log := log.FromContext(ctx)
	statuses, err := r.config.EntryClient.CreateEntries(ctx, entriesFromDeclaredEntries(declaredEntries))
	if err != nil {
		for _, declaredEntry := range declaredEntries {
			declaredEntry.By.IncrementEntryFailures()
		}
		log.Error(err, "Failed to update entries")
		return
	}
	for i, status := range statuses {
		switch status.Code {
		case codes.OK:
			log.Info("Created entry", entryLogFields(declaredEntries[i].Entry)...)
			declaredEntries[i].By.IncrementEntrySuccess()
		default:
			declaredEntries[i].By.IncrementEntryFailures()
			log.Error(status.Err(), "Failed to create entry", entryLogFields(declaredEntries[i].Entry)...)
		}
	}
}

func (r *entryReconciler) updateEntries(ctx context.Context, declaredEntries []declaredEntry) {
	log := log.FromContext(ctx)
	statuses, err := r.config.EntryClient.UpdateEntries(ctx, entriesFromDeclaredEntries(declaredEntries))
	if err != nil {
		for _, declaredEntry := range declaredEntries {
			declaredEntry.By.IncrementEntryFailures()
		}
		log.Error(err, "Failed to update entries")
		return
	}
	for i, status := range statuses {
		switch status.Code {
		case codes.OK:
			log.Info("Updated entry", entryLogFields(declaredEntries[i].Entry)...)
		default:
			declaredEntries[i].By.IncrementEntryFailures()
			log.Error(status.Err(), "Failed to update entry", entryLogFields(declaredEntries[i].Entry)...)
		}
	}
}

func (r *entryReconciler) deleteEntries(ctx context.Context, entries []spireapi.Entry) {
	log := log.FromContext(ctx)
	statuses, err := r.config.EntryClient.DeleteEntries(ctx, idsFromEntries(entries))
	if err != nil {
		log.Error(err, "Failed to delete entries")
		return
	}
	for i, status := range statuses {
		switch status.Code {
		case codes.OK:
			log.Info("Deleted entry", entryLogFields(entries[i])...)
		default:
			log.Error(status.Err(), "Failed to delete entry", entryLogFields(entries[i])...)
		}
	}
}

type entriesState map[entryKey]*entryState

func (es entriesState) AddCurrent(entry spireapi.Entry) {
	s := es.stateFor(entry)
	s.Current = append(s.Current, entry)
}

func (es entriesState) AddDeclared(entry spireapi.Entry, by byObject) {
	s := es.stateFor(entry)
	s.Declared = append(s.Declared, declaredEntry{
		Entry: entry,
		By:    by,
	})
}

func (es entriesState) stateFor(entry spireapi.Entry) *entryState {
	key := makeEntryKey(entry)
	s, ok := es[key]
	if !ok {
		s = &entryState{}
		es[key] = s
	}
	return s
}

type entryState struct {
	Current  []spireapi.Entry
	Declared []declaredEntry
}

type declaredEntry struct {
	Entry spireapi.Entry
	By    byObject
}

type entryKey string

func makeEntryKey(entry spireapi.Entry) entryKey {
	h := sha256.New()
	_, _ = io.WriteString(h, entry.SPIFFEID.String())
	_, _ = io.WriteString(h, entry.ParentID.String())
	for _, selector := range sortSelectors(entry.Selectors) {
		_, _ = io.WriteString(h, selector.Type)
		_, _ = io.WriteString(h, selector.Value)
	}
	sum := h.Sum(nil)
	return entryKey(hex.EncodeToString(sum))
}

func sortSelectors(unsorted []spireapi.Selector) []spireapi.Selector {
	sorted := append([]spireapi.Selector(nil), unsorted...)
	sort.Slice(sorted, func(i, j int) bool {
		switch {
		case sorted[i].Type < sorted[j].Type:
			return true
		case sorted[i].Type > sorted[j].Type:
			return false
		default:
			return sorted[i].Value < sorted[j].Value
		}
	})
	return sorted
}

func sortDeclaredEntriesByPreference(entries []declaredEntry) {
	// The most preferred is sorted to the first slot.
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i].By, entries[j].By
		return objectCmp(a, b) < 0
	})
}

func objectCmp(a, b byObject) int {
	// Sort ascending by creation timestamp
	creationDiff := a.GetCreationTimestamp().UnixNano() - b.GetCreationTimestamp().UnixNano()
	switch {
	case creationDiff < 0:
		return -1
	case creationDiff > 0:
		return 1
	}

	// Sort _descending_ by deletion timestamp (those with no timestamp sort first)
	switch {
	case a.GetDeletionTimestamp() == nil && b.GetDeletionTimestamp() == nil:
		// fallthrough to next criteria
	case a.GetDeletionTimestamp() != nil && b.GetDeletionTimestamp() == nil:
		return 1
	case a.GetDeletionTimestamp() == nil && b.GetDeletionTimestamp() != nil:
		return -1
	case a.GetDeletionTimestamp() != nil && b.GetDeletionTimestamp() != nil:
		deleteDiff := a.GetDeletionTimestamp().UnixNano() - b.GetDeletionTimestamp().UnixNano()
		switch {
		case deleteDiff < 0:
			return 1
		case deleteDiff > 0:
			return -1
		}
	}

	// At this point, these two entries are more or less equal in
	// precedence, but we need a stable sorting mechanism, so tie-break
	// with the UID.
	switch {
	case a.GetUID() < b.GetUID():
		return -1
	case a.GetUID() > b.GetUID():
		return 1
	default:
		return 0
	}
}

func getOutdatedEntryFields(newEntry, oldEntry spireapi.Entry, unsupportedFields map[spireapi.Field]struct{}) []spireapi.Field {
	// We don't need to bother with the parent ID, the SPIFFE ID, or the
	// selectors since they are part of the uniqueness check that resulted in
	// the AlreadyExists error code.
	var outdated []spireapi.Field
	if oldEntry.X509SVIDTTL != newEntry.X509SVIDTTL {
		outdated = append(outdated, spireapi.X509SVIDTTL)
	}
	if oldEntry.JWTSVIDTTL != newEntry.JWTSVIDTTL {
		if _, ok := unsupportedFields[spireapi.JWTSVIDTTLField]; !ok {
			outdated = append(outdated, spireapi.JWTSVIDTTLField)
		}
	}
	if !trustDomainsMatch(oldEntry.FederatesWith, newEntry.FederatesWith) {
		outdated = append(outdated, spireapi.FederatesWithField)
	}
	if oldEntry.Admin != newEntry.Admin {
		outdated = append(outdated, spireapi.AdminField)
	}
	if oldEntry.Downstream != newEntry.Downstream {
		outdated = append(outdated, spireapi.DownstreamField)
	}
	if !stringsMatch(oldEntry.DNSNames, newEntry.DNSNames) {
		outdated = append(outdated, spireapi.DNSNamesField)
	}
	if oldEntry.Hint != newEntry.Hint {
		if _, ok := unsupportedFields[spireapi.HintField]; !ok {
			outdated = append(outdated, spireapi.HintField)
		}
	}
	if oldEntry.StoreSVID != newEntry.StoreSVID {
		if _, ok := unsupportedFields[spireapi.StoreSVIDField]; !ok {
			outdated = append(outdated, spireapi.StoreSVIDField)
		}
	}

	return outdated
}

func trustDomainsMatch(as, bs []spiffeid.TrustDomain) bool {
	if len(as) != len(bs) {
		return false
	}
	// copy the sort the slices
	as = append([]spiffeid.TrustDomain(nil), as...)
	sort.Slice(as, func(i, j int) bool {
		return as[i].Compare(as[j]) < 0
	})
	bs = append([]spiffeid.TrustDomain(nil), bs...)
	sort.Slice(bs, func(i, j int) bool {
		return bs[i].Compare(bs[j]) < 0
	})
	for i := range as {
		if as[i] != bs[i] {
			return false
		}
	}
	return true
}

func stringsMatch(as, bs []string) bool {
	if len(as) != len(bs) {
		return false
	}
	// copy the sort the slices
	as = append([]string(nil), as...)
	sort.Strings(as)
	bs = append([]string(nil), bs...)
	sort.Strings(bs)
	for i := range as {
		if as[i] != bs[i] {
			return false
		}
	}
	return true
}

func entriesFromDeclaredEntries(declaredEntries []declaredEntry) []spireapi.Entry {
	entries := make([]spireapi.Entry, 0, len(declaredEntries))
	for _, declaredEntry := range declaredEntries {
		entries = append(entries, declaredEntry.Entry)
	}
	return entries
}

func idsFromEntries(entries []spireapi.Entry) []string {
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.ID)
	}
	return ids
}

// filterJoinTokenEntries filters out entries that correspond to join tokens.
func filterJoinTokenEntries(entries []spireapi.Entry) []spireapi.Entry {
	if len(entries) == 0 {
		return entries
	}
	filteredEntries := make([]spireapi.Entry, 0, len(entries))
	for _, entry := range entries {
		if isJoinTokenEntry(entry) {
			continue
		}
		filteredEntries = append(filteredEntries, entry)
	}
	return filteredEntries
}

// isJoinTokenEntry returns true if the entry corresponds to a join token.
// For an entry to correspond to a join token, both the following conditions must be true:
// 1. The path of the parent ID of the entry must begin with "/spire/agent/join_token/".
// 2. The entry must contain a selector of type "spiffe_id".
func isJoinTokenEntry(entry spireapi.Entry) bool {
	if !strings.HasPrefix(entry.ParentID.Path(), joinTokenSpiffePrefix) {
		return false
	}
	for _, selector := range entry.Selectors {
		if selector.Type == joinTokenSelectorType {
			return true
		}
	}
	return false
}
