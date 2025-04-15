package metrics

import (
	"context"
	"net/http"
    "os"
    "os/signal"
    "syscall"
	"log"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Define metrics
var (
	// HTTPRequestCounter = prometheus.NewCounterVec(
	// 	prometheus.CounterOpts{
	// 		Name: "http_requests_total",
	// 		Help: "Number of HTTP requests received",
	// 	},
	// 	[]string{"path"},
	// )
	
    // HTTPRequestDuration = prometheus.NewHistogram(
    //     prometheus.HistogramOpts{
    //         Name:    "http_request_duration_seconds",
    //         Help:    "Duration of HTTP requests in seconds",
    //         Buckets: prometheus.DefBuckets,
    //     },
    // )

	FailedCryptoPriceLookupCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "failed_crypto_price_lookup_total",
			Help: "Number of failed Crypto Price API lookups",
		},
		[]string{"coin"},
	)
	
    MessagesProducedCounter = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name:    "kafka_messages_total",
            Help:    "Number of Kafka messages produced",
        },
		[]string{"coin"},
    )
)

// Register metrics
func Init(cancel context.CancelFunc) {
	// prometheus.MustRegister(HTTPRequestCounter)
	// prometheus.MustRegister(HTTPRequestDuration)
	prometheus.MustRegister(FailedCryptoPriceLookupCounter)
	prometheus.MustRegister(MessagesProducedCounter)

    // Handle graceful shutdown
    go handleSignals(cancel)

    // Start metrics endpoint
    go func() {
        http.Handle("/metrics", promhttp.Handler())
        log.Println("Prometheus metrics at :2112/metrics")
        log.Fatal(http.ListenAndServe(":2112", nil))
    }()

}

// Handle OS signals
func handleSignals(cancel context.CancelFunc) {
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan
    log.Println("Received shutdown signal")
    cancel()
}