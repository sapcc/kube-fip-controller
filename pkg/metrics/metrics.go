package metrics

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const metricNamespace = "kube_fip_controller"

var (
	// MetricErrorAssociateInstanceAndFIP ...
	MetricErrorAssociateInstanceAndFIP = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: metricNamespace,
		Name:      "associate_instance_fip_errors_total",
		Help:      "Counter for associating instance and FIP errors.",
	})

	// MetricErrorCreateFIP ...
	MetricErrorCreateFIP = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: metricNamespace,
		Name:      "create_fip_errors_total",
		Help:      "Counter for creating FIP errors.",
	})

	// MetricSuccessfulOperations ...
	MetricSuccessfulOperations = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: metricNamespace,
		Name:      "successful_operations_total",
		Help:      "Counter for successful operations.",
	})

	MetricFailedOperations = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: metricNamespace,
		Name:      "failed_operations_total",
		Help:      "Counter for failed operations.",
	})
)

func init() {
	prometheus.MustRegister(
		MetricErrorAssociateInstanceAndFIP,
		MetricErrorCreateFIP,
		MetricSuccessfulOperations,
		MetricFailedOperations,
	)
}

// ServeMetrics starts the Prometheus metrics collector.
func ServeMetrics(host net.IP, port int, wg *sync.WaitGroup, stop <-chan struct{}, logger log.Logger) {
	wg.Add(1)
	defer wg.Done()

	logger = log.With(logger, "component", "metrics")

	addr := fmt.Sprintf("%s:%d", host.String(), port)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		//nolint:errcheck
		_ = level.Error(logger).Log("msg", "failed serve prometheus metrics", "err", err)
		return
	}
	defer func(l net.Listener) {
		err := l.Close()
		if err != nil {
			//nolint:errcheck
			_ = level.Error(logger).Log("msg", "failed to close listener", "err", err)
		}
	}(l)
	//nolint:errcheck
	_ = level.Info(logger).Log("msg", "serving prometheus metrics", "address", addr, "path", "/metrics")

	go func() {
		server := &http.Server{
			ReadHeaderTimeout: 5 * time.Second,
		}
		server.Handler = promhttp.Handler()
		err = server.Serve(l)
		if err != nil {
			//nolint:errcheck
			_ = level.Error(logger).Log("msg", "failed to serve prometheus metrics", "err", err)
			if err != nil {
				return
			}
		}
	}()
	<-stop
}
