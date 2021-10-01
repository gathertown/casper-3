// Metrics package to export useful metrics for monitoring and alerting.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var namespace = "casper3"

// define custom metrics
// https://pkg.go.dev/github.com/prometheus/client_golang@v1.10.0/prometheus#GaugeVec
var (
	executionError = promauto.NewCounterVec(prometheus.CounterOpts{
		Name:      "execution_error",
		Namespace: namespace,
		Subsystem: "app",
		Help:      "Execution errors ecountered",
	},
		[]string{"errorMessage"},
	)

	dnsRecordTotal = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name:      "records_total",
		Namespace: namespace,
		Subsystem: "dns",
		Help:      "Total amount of DNS records for the provider",
	},
		[]string{"provider"},
	)
)

func ExecErrInc(msg string) {
	executionError.WithLabelValues(msg).Inc()
}

func DNSRecordsTotal(provider string, n float64) {
	dnsRecordTotal.WithLabelValues(provider).Set(n)
}

func Serve() {
	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":8080", nil)
}
