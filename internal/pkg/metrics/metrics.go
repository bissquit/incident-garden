// Package metrics provides Prometheus metrics definitions.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const namespace = "incidentgarden"

var (
	// HTTPRequestDuration tracks HTTP request latency.
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Buckets:   []float64{.01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30},
		},
		[]string{"method", "route", "status_code"},
	)

	// DBPoolConnections tracks database connection pool state.
	DBPoolConnections = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "db",
			Name:      "pool_connections",
			Help:      "Number of database connections by state",
		},
		[]string{"state"},
	)
)
