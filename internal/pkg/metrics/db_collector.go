package metrics

import (
	"github.com/jackc/pgx/v5/pgxpool"
)

// RecordDBPoolMetrics updates database pool metrics.
func RecordDBPoolMetrics(pool *pgxpool.Pool) {
	stats := pool.Stat()

	DBPoolConnections.WithLabelValues("in_use").Set(float64(stats.AcquiredConns()))
	DBPoolConnections.WithLabelValues("idle").Set(float64(stats.IdleConns()))
	DBPoolConnections.WithLabelValues("max").Set(float64(stats.MaxConns()))
}
