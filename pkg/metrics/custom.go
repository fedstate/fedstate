package metrics

import (
	"fmt"
	"net/http"

	"github.com/fedstate/fedstate/pkg/logi"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	metricsNamespace        = "mongo_operator"
	promControllerSubsystem = "mongo_operator_controller"
)

var (
	CustomMetricsPort   int32  = 8989
	OperatorMetricsAddr string = ":8443"
	log                        = logi.Log.Sugar()
)

func init() {
	MetricClient = NewPrometheusMetrics("/metrics", http.DefaultServeMux, prometheus.NewRegistry())
}

// 自定义operator cr监控信息
func ServeCustomMetrics() {
	go func() {
		addr := fmt.Sprintf(":%d", CustomMetricsPort)
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Errorf("server custom metric error: %v", err.Error())
			panic(err)
		}
	}()
}

var MetricClient Instrumenter

// Instrumenter is the interface that will collect the metrics and has ability to send/expose those metrics.
type Instrumenter interface {
	IncMongoReconcileError(namespace string, name string)
}

// PromMetrics implements the instrumenter so the metrics can be managed by Prometheus.
type PromMetrics struct {
	// Metrics fields.
	reconcileErrorCount *prometheus.CounterVec
	// Instrumentation fields.
	registry prometheus.Registerer
}

// NewPrometheusMetrics returns a new PromMetrics object.
func NewPrometheusMetrics(path string, mux *http.ServeMux, registry *prometheus.Registry) *PromMetrics {
	// Create metrics.
	mongoOperatorReconcileErrorCount := prometheus.NewCounterVec(prometheus.CounterOpts{ // Namespace+Subsystem+Name 组成指标名
		Namespace: metricsNamespace,
		Subsystem: promControllerSubsystem,
		Name:      "reconcile_error",
		Help:      "Reconcile error num of mongo operator",
	}, []string{"namespace", "name"})

	// Create the instance.
	p := &PromMetrics{
		reconcileErrorCount: mongoOperatorReconcileErrorCount,
		registry:            registry,
	}

	// Register metrics on prometheus.
	p.register()

	// Register prometheus handler so we can serve the metrics.
	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	mux.Handle(path, handler)

	return p
}

// register will register all the required prometheus metrics on the Prometheus collector.
func (p *PromMetrics) register() {
	p.registry.MustRegister(p.reconcileErrorCount)
}

func (p *PromMetrics) IncMongoReconcileError(namespace string, name string) {
	p.reconcileErrorCount.WithLabelValues(namespace, name).Inc()
}
