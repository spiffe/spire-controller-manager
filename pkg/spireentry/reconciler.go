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
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"slices"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"google.golang.org/grpc/codes"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	spirev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	"github.com/spiffe/spire-controller-manager/pkg/k8sapi"
	"github.com/spiffe/spire-controller-manager/pkg/metrics"
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
	TrustDomain              spiffeid.TrustDomain
	ClusterName              string
	ClusterDomain            string
	EntryClient              spireapi.EntryClient
	K8sClient                client.Client
	IgnoreNamespaces         []*regexp.Regexp
	AutoPopulateDNSNames     bool
	ClassName                string
	WatchClassless           bool
	ParentIDTemplate         *template.Template
	Reconcile                spirev1alpha1.ReconcileConfig
	EntryIDPrefix            string
	EntryIDPrefixCleanup     *string
	StaticManifestPath       *string
	ExpandEnvStaticManifests bool

	// GCInterval how long to sit idle (i.e. untriggered) before doing
	// another reconcile.
	GCInterval time.Duration

	// EntryRenderCacheSize is the maximum number of entries in the LRU cache
	// for rendered pod entries. If zero, defaults to defaultEntryRenderCacheSize.
	EntryRenderCacheSize int
}

const (
	// defaultEntryRenderCacheSize is the default size for the LRU cache
	defaultEntryRenderCacheSize = 300000
)

func Reconciler(config ReconcilerConfig) reconciler.Reconciler {
	r := &entryReconciler{
		config:                   config,
		promCounter:              metrics.PromCounters,
		staticManifestPath:       config.StaticManifestPath,
		expandEnvStaticManifests: config.ExpandEnvStaticManifests,
	}

	cacheSize := config.EntryRenderCacheSize
	if cacheSize <= 0 {
		cacheSize = defaultEntryRenderCacheSize
	}
	renderCache, err := lru.New[string, *cachedEntry](cacheSize)
	if err != nil {
		log.Log.WithName("entry-reconciler").Error(err, "Failed to create entry cache, running without cache")
	} else {
		r.renderCache = renderCache
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
	promCounter              map[string]prometheus.Counter
	nextGetUnsupportedFields time.Time
	staticManifestPath       *string
	expandEnvStaticManifests bool

	renderCache *lru.Cache[string, *cachedEntry] // thread-safe LRU cache for pod entries

}

type cachedEntry struct {
	entry       *spireapi.Entry
	podRV       string // Pod RV
	nodeRV      string // Node RV
	specHash    string // Hash of ClusterSPIFFEID spec
	endpointsRV string // Concatenated Endpoints RVs
}

// isValid checks whether the cached entry is still fresh by comparing
// the RVs and hashes against the current state.
func (c *cachedEntry) isValid(podRV, nodeRV, specHash, endpointsRV string) bool {
	return c.podRV == podRV &&
		c.nodeRV == nodeRV &&
		c.specHash == specHash &&
		c.endpointsRV == endpointsRV
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
	if r.config.Reconcile.ClusterStaticEntries {
		// Load and add entry state for ClusterStaticEntries
		clusterStaticEntries, err = r.listClusterStaticEntries(ctx, r.expandEnvStaticManifests)
		if err != nil {
			log.Error(err, "Failed to list ClusterStaticEntries")
			return
		}
		r.addClusterStaticEntryEntriesState(ctx, state, clusterStaticEntries)
	}

	clusterSPIFFEIDs := []*ClusterSPIFFEID{}
	if r.config.Reconcile.ClusterSPIFFEIDs {
		// Load and add entry state for ClusterSPIFFEIDs
		clusterSPIFFEIDs, err = r.listClusterSPIFFEIDs(ctx)
		if err != nil {
			log.Error(err, "Failed to list ClusterSPIFFEIDs")
			return
		}

		// Pre-load all nodes into a map to avoid per-pod Get() calls,
		// which each incur a deep copy and mutex lock on the informer cache.
		nodeMap, err := r.buildNodeMap(ctx)
		if err != nil {
			log.Error(err, "Failed to list nodes")
			return
		}
		r.addClusterSPIFFEIDEntriesState(ctx, state, clusterSPIFFEIDs, nodeMap)
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

func (r *entryReconciler) listClusterStaticEntries(ctx context.Context, expandEnv bool) ([]*ClusterStaticEntry, error) {
	var clusterStaticEntries []spirev1alpha1.ClusterStaticEntry
	var err error
	if r.config.K8sClient != nil {
		clusterStaticEntries, err = k8sapi.ListClusterStaticEntries(ctx, r.config.K8sClient)
	} else {
		clusterStaticEntries, err = spirev1alpha1.ListClusterStaticEntries(ctx, *r.staticManifestPath, expandEnv)
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

func (r *entryReconciler) buildNodeMap(ctx context.Context) (map[string]*corev1.Node, error) {
	nodes, err := k8sapi.ListNodes(ctx, r.config.K8sClient)
	if err != nil {
		return nil, err
	}
	nodeMap := make(map[string]*corev1.Node, len(nodes))
	for i := range nodes {
		nodeMap[nodes[i].Name] = &nodes[i]
	}
	return nodeMap, nil
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
			r.promCounter[metrics.StaticEntryFailures].Add(1)
			continue
		}
		clusterStaticEntry.NextStatus.Rendered = true
		state.AddDeclared(*entry, clusterStaticEntry)
	}
}

func (r *entryReconciler) addClusterSPIFFEIDEntriesState(ctx context.Context, state entriesState, clusterSPIFFEIDs []*ClusterSPIFFEID, nodeMap map[string]*corev1.Node) {
	log := log.FromContext(ctx)
	podsWithNonFallbackApplied := make(map[types.UID]struct{})
	// Process all the fallback clusterSPIFFEIDs last.
	slices.SortStableFunc(clusterSPIFFEIDs, func(x, y *ClusterSPIFFEID) int {
		if x.Spec.Fallback == y.Spec.Fallback {
			return 0
		}
		if x.Spec.Fallback {
			return 1
		}
		return -1
	})
	for _, clusterSPIFFEID := range clusterSPIFFEIDs {
		log := log.WithValues(clusterSPIFFEIDLogKey, objectName(clusterSPIFFEID))

		spec, err := spirev1alpha1.ParseClusterSPIFFEIDSpec(&clusterSPIFFEID.Spec)
		if err != nil {
			// TODO: should this be prevented via admission webhook? should
			// we dump this failure into the status?
			log.Error(err, "Failed to parse ClusterSPIFFEID spec")
			continue
		}

		// Compute spec hash once for all pods in this ClusterSPIFFEID
		// This avoids recalculating the same hash for every pod
		specHash, err := computeObjectHash(spec)
		if err != nil {
			log.Error(err, "Failed to hash ClusterSPIFFEID spec")
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
				if _, ok := podsWithNonFallbackApplied[pods[i].UID]; ok && clusterSPIFFEID.Spec.Fallback {
					continue
				}

				entry, err := r.renderPodEntry(ctx, spec, &pods[i], specHash, nodeMap)
				switch {
				case err != nil:
					log.Error(err, "Failed to render entry")
					clusterSPIFFEID.NextStatus.Stats.PodEntryRenderFailures++
				case entry != nil:
					// renderPodEntry will return a nil entry if requisite k8s
					// objects disappeared from underneath.
					state.AddDeclared(*entry, clusterSPIFFEID)
					if !clusterSPIFFEID.Spec.Fallback {
						podsWithNonFallbackApplied[pods[i].UID] = struct{}{}
					}
				}
			}
		}
	}
}

// podEntryCacheKey generates a cache key for a pod entry based on pod UID.
// We use UID alone (not UID+ResourceVersion+xxxHash) as the key so that each pod
// occupies exactly one LRU slot. ResourceVersions and other volatile fields
// are checked separately via cachedEntry.isValid; this avoids accumulating
// stale entries in the cache when pods or nodes are updated.
func podEntryCacheKey(podUID types.UID) string {
	return string(podUID)
}

// computeObjectHash computes a SHA256 hash of an object by serializing it to JSON.
func computeObjectHash(obj interface{}) (string, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}

// computeEndpointsRV builds a compact string from individual Endpoint
// ResourceVersions. This is much cheaper than JSON-serializing the full objects
// and is sufficient for change detection since RV changes on any mutation.
func computeEndpointsRV(items []corev1.Endpoints) string {
	var sb strings.Builder
	for i := range items {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(items[i].ResourceVersion)
	}
	return sb.String()
}

func (r *entryReconciler) renderPodEntry(ctx context.Context, spec *spirev1alpha1.ParsedClusterSPIFFEIDSpec, pod *corev1.Pod, specHash string, nodeMap map[string]*corev1.Node) (*spireapi.Entry, error) {
	// Get node from cache map instead of making per-pod Get() calls
	node, ok := nodeMap[pod.Spec.NodeName]
	if !ok {
		return nil, fmt.Errorf("node %s not found in cache", pod.Spec.NodeName)
	}

	endpointsList := &corev1.EndpointsList{}
	endpointsRV := ""
	if spec.AutoPopulateDNSNames {
		if err := r.config.K8sClient.List(ctx, endpointsList, client.InNamespace(pod.Namespace), client.MatchingFields{reconciler.EndpointUID: string(pod.UID)}); err != nil && !apierrors.IsNotFound(err) {
			return nil, err
		}
		if len(endpointsList.Items) > 0 {
			endpointsRV = computeEndpointsRV(endpointsList.Items)
		}
	}
	// 1. Get rendered entry from cache
	cacheKey := podEntryCacheKey(pod.UID)
	if r.renderCache != nil {
		if cached, ok := r.renderCache.Get(cacheKey); ok && cached.isValid(pod.ResourceVersion, node.ResourceVersion, specHash, endpointsRV) {
			return cached.entry, nil
		}
	}
	// 2. Perform render entry if cache miss
	entry, err := renderPodEntry(spec, node, pod, endpointsList, r.config.TrustDomain, r.config.ClusterName, r.config.ClusterDomain, r.config.ParentIDTemplate)
	if err != nil {
		return nil, err
	}
	// 3. Cache the entry
	if r.renderCache != nil {
		r.renderCache.Add(cacheKey, &cachedEntry{
			specHash:    specHash,
			nodeRV:      node.ResourceVersion,
			podRV:       pod.ResourceVersion,
			endpointsRV: endpointsRV,
			entry:       entry,
		})
	}
	return entry, nil
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
