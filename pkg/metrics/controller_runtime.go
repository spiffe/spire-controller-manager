package metrics

import "github.com/prometheus/client_golang/prometheus"

const (
	StaticEntryFailures = "cluster_static_entry_failures"
)

var (
	PromCounters = map[string]prometheus.Counter{
		StaticEntryFailures: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: StaticEntryFailures,
				Help: "Number of cluster static entry render failures",
			},
		),
	}
)
