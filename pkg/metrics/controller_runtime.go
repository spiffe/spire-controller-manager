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

package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// ---- Legacy counter (kept for backward compatibility) ----

const StaticEntryFailures = "cluster_static_entry_failures"

// PromCounters exposes named counters used by legacy code paths.
// New code should use the typed package-level variables directly.
var PromCounters = map[string]prometheus.Counter{
	StaticEntryFailures: prometheus.NewGauge(prometheus.GaugeOpts{
		Name: StaticEntryFailures,
		Help: "Number of cluster static entry render failures",
	}),
}

// ---- Metric namespace ----

const namespace = "spire_controller_manager"

// ---- Phase label constants ----

const (
	PhaseListEntries        = "list_entries"
	PhaseListStaticEntries  = "list_static_entries"
	PhaseListClusterSPIFFEIDs = "list_cluster_spiffeids"
	PhaseBuildNodeMap       = "build_node_map"
	PhaseRenderState        = "render_state"
	PhaseDiff               = "diff"
	PhaseDelete             = "delete"
	PhaseCreate             = "create"
	PhaseUpdate             = "update"
	PhaseStatusUpdate       = "status_update"
)

// ---- Trigger label constants ----

const (
	TriggerTriggered = "triggered"
	TriggerPeriodic  = "periodic"
)

// ---- Result label constants ----

const (
	ResultOK    = "ok"
	ResultError = "error"
)

// ---- Code label constants (gRPC status codes mapped to human-readable labels) ----

const (
	CodeOK            = "ok"
	CodeAlreadyExists = "already_exists"
	CodeNotFound      = "not_found"
	CodeError         = "error"
)

// ---- Operation label constants (SPIRE API operations) ----

const (
	OpListEntries = "list_entries"
	OpBatchCreate = "batch_create"
	OpBatchUpdate = "batch_update"
	OpBatchDelete = "batch_delete"
)

// ---- Render cache result constants ----

const (
	CacheHit  = "hit"
	CacheMiss = "miss"
)

// ---- Metric declarations ----

// ReconcileDuration measures the wall-clock duration of each full reconcile pass.
var ReconcileDuration = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "reconcile_duration_seconds",
		Help: "Wall-clock duration of a complete reconcile pass. " +
			"Labels: kind (entry|federation), trigger (triggered|periodic), result (ok|error). " +
			"The p90/p99 tail directly measures the pod-attestation delay ceiling.",
		Buckets: []float64{0.5, 1, 5, 10, 30, 60, 120, 180, 300},
	},
	[]string{"kind", "trigger", "result"},
)

// ReconcilePhaseDuration measures each named sub-phase of a reconcile pass.
// Use this to identify which phase is the bottleneck (list_entries vs create vs delete, etc.).
var ReconcilePhaseDuration = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "reconcile_phase_duration_seconds",
		Help: "Wall-clock duration of each named sub-phase within a reconcile pass. " +
			"Phases: list_entries, list_static_entries, list_cluster_spiffeids, build_node_map, " +
			"render_state, diff, delete, create, update, status_update.",
		Buckets: []float64{0.05, 0.1, 0.5, 1, 5, 10, 30, 60, 120, 180},
	},
	[]string{"phase"},
)

// SPIREAPIRequestDuration measures the latency of individual SPIRE server API calls.
// This is the primary instrument for confirming the datastore is the bottleneck: a 22s
// per-batch value here points squarely at the SPIRE server / its SQL datastore.
var SPIREAPIRequestDuration = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "spire_api_request_duration_seconds",
		Help: "Wall-clock latency of individual SPIRE server API calls, by operation " +
			"(list_entries, batch_create, batch_update, batch_delete). " +
			"High values here confirm the SPIRE server / datastore is the bottleneck, not the controller.",
		Buckets: []float64{0.05, 0.1, 0.5, 1, 5, 10, 30, 60, 120},
	},
	[]string{"operation"},
)

// EntryWriteTotal counts every SPIRE entry write result by operation and status code.
// The already_exists and not_found codes represent wasted work from double execution when
// leader election is off — tracking these quantifies the benefit of enabling leader election.
var EntryWriteTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "spire_entry_write_total",
		Help: "Total SPIRE entry write results, by operation (batch_create|batch_update|batch_delete) " +
			"and gRPC status code (ok|already_exists|not_found|error). " +
			"already_exists on create and not_found on delete indicate wasted work from double execution.",
	},
	[]string{"operation", "code"},
)

// ReconcileTriggerTotal counts Trigger() calls by whether the trigger was enqueued or dropped.
// A high drop rate means reconcile passes are running continuously with no idle time; triggers
// fired while a pass is in flight are silently coalesced (dropped).
var ReconcileTriggerTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "reconcile_trigger_total",
		Help: "Total reconcile Trigger() calls, by result (enqueued|dropped). " +
			"Dropped triggers occur when a pass is already in flight; a high drop rate " +
			"means new pod events are being discarded and latency will be >= one full pass.",
	},
	[]string{"result"},
)

// EntryRenderCacheTotal counts render cache lookups.
// Use this to confirm the render cache is effective (high hit rate) and to distinguish
// cache-warm from cache-cold pass behaviour.
var EntryRenderCacheTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "entry_render_cache_total",
		Help: "Total entry render cache lookups, by result (hit|miss). " +
			"A high miss rate on a warm cluster means pods/nodes/CSIDs are changing frequently.",
	},
	[]string{"result"},
)

// PodAttestationDelay is the primary user-facing SLI: time from pod creation to the moment
// its SPIRE registration entry is successfully created (codes.OK only, not AlreadyExists).
// The Investigation.md example showed a p50 ~14 min delay; alert when this exceeds your SLO.
var PodAttestationDelay = prometheus.NewHistogram(
	prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "pod_attestation_delay_seconds",
		Help: "Time from pod creation (pod.CreationTimestamp) to successful SPIRE registration entry " +
			"creation (codes.OK only). This is the primary user-facing SLI for identity issuance " +
			"latency. Recorded only for genuinely new entries, not AlreadyExists races.",
		Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600, 900},
	},
)

// ReconcileBatchEntries records the size of each write batch at the point it is issued.
var ReconcileBatchEntries = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "reconcile_batch_entries",
		Help: "Number of entries in the most recent write batch, by operation " +
			"(batch_create|batch_update|batch_delete). Useful for correlating batch size with RPC latency.",
	},
	[]string{"operation"},
)

// ---- Registration ----

// Register registers all new metrics with the provided Prometheus Registerer.
// Call once at startup alongside PromCounters registration.
func Register(reg prometheus.Registerer) {
	reg.MustRegister(
		ReconcileDuration,
		ReconcilePhaseDuration,
		SPIREAPIRequestDuration,
		EntryWriteTotal,
		ReconcileTriggerTotal,
		EntryRenderCacheTotal,
		PodAttestationDelay,
		ReconcileBatchEntries,
	)
}

// ---- Helpers ----

// ObservePhase returns a stop function that, when called, records the elapsed
// duration of the named reconcile phase into ReconcilePhaseDuration. Typical usage:
//
//	stop := metrics.ObservePhase(metrics.PhaseListEntries)
//	// ... do the work ...
//	stop()
func ObservePhase(phase string) func() {
	start := time.Now()
	return func() {
		ReconcilePhaseDuration.WithLabelValues(phase).Observe(time.Since(start).Seconds())
	}
}
