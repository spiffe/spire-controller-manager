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
	"io"
	"regexp"
	"sort"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	spirev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	"github.com/spiffe/spire-controller-manager/pkg/k8sapi"
	"github.com/spiffe/spire-controller-manager/pkg/reconciler"
	"github.com/spiffe/spire-controller-manager/pkg/spireapi"
	"google.golang.org/grpc/codes"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ReconcilerConfig struct {
	TrustDomain      spiffeid.TrustDomain
	ClusterName      string
	ClusterDomain    string
	EntryClient      spireapi.EntryClient
	K8sClient        client.Client
	IgnoreNamespaces []*regexp.Regexp

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
}

func (r *entryReconciler) reconcile(ctx context.Context) {
	log := log.FromContext(ctx)

	// Load current entries from SPIRE server.
	currentEntries, err := r.listEntries(ctx)
	if err != nil {
		log.Error(err, "Failed to list SPIRE entries")
		return
	}

	// Populate the existing state
	state := make(entriesState)
	for _, entry := range currentEntries {
		state.AddCurrent(entry)
	}

	// Load ClusterSPIFFEIDs
	clusterSPIFFEIDs, err := r.listClusterSPIFFEIDs(ctx)
	if err != nil {
		log.Error(err, "Failed to list ClusterSPIFFEIDs")
		return
	}

	// Build up the list of declared entries
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
			if isNamespaceIgnored(r.config.IgnoreNamespaces, namespaces[i].Name) {
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

	var toDelete []spireapi.Entry
	var toCreate []declaredEntry
	var toUpdate []declaredEntry

	for _, s := range state {
		// Sort declared entries.
		sortDeclaredEntriesByPreference(s.Declared)
		if len(s.Declared) > 0 {
			// Grab the first to set.
			preferredEntry := s.Declared[0]
			preferredEntry.By.NextStatus.Stats.EntriesToSet++

			// Record the remaining as masked.
			for _, otherEntry := range s.Declared[1:] {
				otherEntry.By.NextStatus.Stats.EntriesMasked++
			}

			// Borrow the current entry ID if available, for the update. Then
			// drop the current entry from the list so it isn't added to the
			// "to delete" list.
			if len(s.Current) == 0 {
				toCreate = append(toCreate, preferredEntry)
			} else {
				preferredEntry.Entry.ID = s.Current[0].ID
				if outdatedFields := getOutdatedEntryFields(preferredEntry.Entry, s.Current[0]); len(outdatedFields) != 0 {
					// Current field does not match. Nothing to do.
					toUpdate = append(toUpdate, preferredEntry)
				}
				s.Current = s.Current[1:]
			}
		}

		// Any remaining current entries should be removed that aren't going
		// to be reused for the entry update.
		toDelete = append(toDelete, s.Current...)
	}

	if len(toDelete) > 0 {
		r.deleteEntries(ctx, toDelete)
	}
	if len(toCreate) > 0 {
		r.createEntries(ctx, toCreate)
	}
	if len(toUpdate) > 0 {
		r.updateEntries(ctx, toUpdate)
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

func (r *entryReconciler) listEntries(ctx context.Context) ([]spireapi.Entry, error) {
	// TODO: cache?
	return r.config.EntryClient.ListEntries(ctx)
}

func (r *entryReconciler) listClusterSPIFFEIDs(ctx context.Context) ([]*ClusterSPIFFEID, error) {
	clusterSPIFFEIDs, err := k8sapi.ListClusterSPIFFEIDs(ctx, r.config.K8sClient)
	if err != nil {
		return nil, err
	}
	out := make([]*ClusterSPIFFEID, 0, len(clusterSPIFFEIDs))
	for _, clusterSPIFFEID := range clusterSPIFFEIDs {
		out = append(out, &ClusterSPIFFEID{
			ClusterSPIFFEID: clusterSPIFFEID,
		})
	}
	return out, nil
}

func (r *entryReconciler) listNamespaces(ctx context.Context, namespaceSelector labels.Selector) ([]corev1.Namespace, error) {
	return k8sapi.ListNamespaces(ctx, r.config.K8sClient, namespaceSelector)
}

func (r *entryReconciler) listNamespacePods(ctx context.Context, namespace string, podSelector labels.Selector) ([]corev1.Pod, error) {
	return k8sapi.ListNamespacePods(ctx, r.config.K8sClient, namespace, podSelector)
}

func (r *entryReconciler) renderPodEntry(ctx context.Context, spec *spirev1alpha1.ParsedClusterSPIFFEIDSpec, pod *corev1.Pod) (*spireapi.Entry, error) {
	// TODO: should we be caching this? probably not since it grabs from the
	// controller client, which is cached already.
	node := new(corev1.Node)
	if err := r.config.K8sClient.Get(ctx, types.NamespacedName{Name: pod.Spec.NodeName}, node); err != nil {
		return nil, client.IgnoreNotFound(err)
	}
	return renderPodEntry(spec, node, pod, r.config.TrustDomain, r.config.ClusterName, r.config.ClusterDomain)
}

func (r *entryReconciler) createEntries(ctx context.Context, declaredEntries []declaredEntry) {
	log := log.FromContext(ctx)
	statuses, err := r.config.EntryClient.CreateEntries(ctx, entriesFromDeclaredEntries(declaredEntries))
	if err != nil {
		for _, declaredEntry := range declaredEntries {
			declaredEntry.By.NextStatus.Stats.EntryFailures++
		}
		log.Error(err, "Failed to update entries")
		return
	}
	for i, status := range statuses {
		switch status.Code {
		case codes.OK:
			log.Info("Created entry", entryLogFields(declaredEntries[i].Entry)...)
		default:
			declaredEntries[i].By.NextStatus.Stats.EntryFailures++
			log.Error(status.Err(), "Failed to create entry", entryLogFields(declaredEntries[i].Entry)...)
		}
	}
}

func (r *entryReconciler) updateEntries(ctx context.Context, declaredEntries []declaredEntry) {
	log := log.FromContext(ctx)
	statuses, err := r.config.EntryClient.UpdateEntries(ctx, entriesFromDeclaredEntries(declaredEntries))
	if err != nil {
		for _, declaredEntry := range declaredEntries {
			declaredEntry.By.NextStatus.Stats.EntryFailures++
		}
		log.Error(err, "Failed to update entries")
		return
	}
	for i, status := range statuses {
		switch status.Code {
		case codes.OK:
			log.Info("Updated entry", entryLogFields(declaredEntries[i].Entry)...)
		default:
			declaredEntries[i].By.NextStatus.Stats.EntryFailures++
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

func (es entriesState) AddDeclared(entry spireapi.Entry, source *ClusterSPIFFEID) {
	s := es.stateFor(entry)
	s.Declared = append(s.Declared, declaredEntry{
		Entry: entry,
		By:    source,
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

type ClusterSPIFFEID struct {
	spirev1alpha1.ClusterSPIFFEID
	NextStatus spirev1alpha1.ClusterSPIFFEIDStatus
}

type entryState struct {
	Current  []spireapi.Entry
	Declared []declaredEntry
}

type declaredEntry struct {
	Entry spireapi.Entry
	By    *ClusterSPIFFEID
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
		a := entries[i].By.ObjectMeta
		b := entries[j].By.ObjectMeta

		// Sort ascending by creation timestamp
		creationDiff := a.CreationTimestamp.UnixNano() - b.CreationTimestamp.UnixNano()
		switch {
		case creationDiff < 0:
			return true
		case creationDiff > 0:
			return false
		}

		// Sort _descending_ by deletion timestamp (those with no timestamp sort first)
		switch {
		case a.DeletionTimestamp == nil && b.DeletionTimestamp == nil:
			// fallthrough to next criteria
		case a.DeletionTimestamp != nil && b.DeletionTimestamp == nil:
			return false
		case a.DeletionTimestamp == nil && b.DeletionTimestamp != nil:
			return true
		case a.DeletionTimestamp != nil && b.DeletionTimestamp != nil:
			deleteDiff := a.DeletionTimestamp.UnixNano() - b.DeletionTimestamp.UnixNano()
			switch {
			case deleteDiff < 0:
				return false
			case deleteDiff > 0:
				return true
			}
		}

		// At this point, these two entries are more or less equal in
		// precedence, but we need a stable sorting mechanism, so tie-break
		// with the UID.
		return a.UID < b.UID
	})
}

func getOutdatedEntryFields(newEntry, oldEntry spireapi.Entry) []string {
	// We don't need to bother with the parent ID, the SPIFFE ID, or the
	// selectors since they are part of the uniqueness check that resulted in
	// the AlreadyExists error code.
	var outdated []string
	if oldEntry.X509SVIDTTL != newEntry.X509SVIDTTL {
		outdated = append(outdated, "x509SVIDTTL")
	}
	if !trustDomainsMatch(oldEntry.FederatesWith, newEntry.FederatesWith) {
		outdated = append(outdated, "federatesWith")
	}
	if oldEntry.Admin != newEntry.Admin {
		outdated = append(outdated, "admin")
	}
	if oldEntry.Downstream != newEntry.Downstream {
		outdated = append(outdated, "downstream")
	}
	if !stringsMatch(oldEntry.DNSNames, newEntry.DNSNames) {
		outdated = append(outdated, "dnsNames")
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

func isNamespaceIgnored(ignoredNamespaces []*regexp.Regexp, namespace string) bool {
	for _, regex := range ignoredNamespaces {
		if regex.MatchString(namespace) {
			return true
		}
	}

	return false
}
