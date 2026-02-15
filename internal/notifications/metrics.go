package notifications

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const namespace = "incidentgarden"

var (
	notificationQueueSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "notifications",
			Name:      "queue_size",
			Help:      "Number of notifications in queue by status",
		},
		[]string{"status"},
	)

	notificationsSent = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "notifications",
			Name:      "sent_total",
			Help:      "Total notifications processed",
		},
		[]string{"channel_type", "status"},
	)

	notificationSendDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "notifications",
			Name:      "send_duration_seconds",
			Help:      "Time to send notification",
			Buckets:   []float64{.01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"channel_type"},
	)

	notificationsProcessed = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "notifications",
			Name:      "queue_fetched_total",
			Help:      "Total notifications fetched from queue (before send attempt). Sum of sent_total should match this.",
		},
	)
)

// recordNotificationSent records a sent notification metric.
func recordNotificationSent(channelType, status string) {
	notificationsSent.WithLabelValues(channelType, status).Inc()
}

// recordNotificationDuration records notification send duration.
func recordNotificationDuration(channelType string, duration time.Duration) {
	notificationSendDuration.WithLabelValues(channelType).Observe(duration.Seconds())
}

// recordQueueProcessed records the number of items processed from queue.
func recordQueueProcessed(count int) {
	notificationsProcessed.Add(float64(count))
}

// RecordQueueStats updates queue size metrics.
func RecordQueueStats(stats *QueueStats) {
	notificationQueueSize.WithLabelValues("pending").Set(float64(stats.Pending))
	notificationQueueSize.WithLabelValues("processing").Set(float64(stats.Processing))
	notificationQueueSize.WithLabelValues("sent").Set(float64(stats.Sent))
	notificationQueueSize.WithLabelValues("failed").Set(float64(stats.Failed))
}
