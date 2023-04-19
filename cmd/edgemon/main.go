package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/sdk/metric"
)

var port = flag.Int("port", 8888, "metrics port")

// This is a Prometheus exporter w/ a single uptime metric.
func main() {
	flag.Parse()

	registry := prometheus.NewRegistry()
	exporter, err := otelprom.New(otelprom.WithRegisterer(registry))
	if err != nil {
		log.Fatal(err)
	}
	provider := metric.NewMeterProvider(metric.WithReader(exporter))
	meter := provider.Meter("github.com/jmacd/caspar.water/cmd/edgemon")

	startTime := time.Now()
	if _, err := meter.Float64ObservableGauge("uptime", instrument.WithUnit("s"), instrument.WithFloat64Callback(func(_ context.Context, o instrument.Float64Observer) error {
		o.Observe(time.Since(startTime).Seconds())
		return nil
	})); err != nil {
		log.Fatal(err)
	}

	http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	if err := http.ListenAndServe(fmt.Sprint(":", *port), nil); err != nil {
		log.Fatal("error serving http: %v", err)
	}
}
