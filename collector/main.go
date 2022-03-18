package main

import (
	"context"
	"log"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/service"
	"go.opentelemetry.io/collector/service/defaultcomponents"

	"github.com/jmacd/caspar.water/sparkplug/sparkplugreceiver"
)

func main() {
	factories, err := defaultcomponents.Components()
	if err != nil {
		log.Fatalf("failed to build components: %v", err)
	}

	spr := sparkplugreceiver.NewFactory()
	factories.Receivers[spr.Type()] = spr

	info := component.BuildInfo{
		Command:     "caspar-water-collector",
		Description: "Caspar Water OpenTelemetry Collector distribution",
		Version:     "0.0.1",
	}

	app, err := service.New(service.CollectorSettings{
		BuildInfo: info,
		Factories: factories,
		ConfigProvider: service.MustNewDefaultConfigProvider(
			[]string{"config.yaml"},
			nil,
		),
	})
	if err != nil {
		log.Fatalf("failed to construct the application: %v", err)
	}

	ctx := context.Background()
	err = app.Run(ctx)
	if err != nil {
		log.Fatalf("application run finished with error: %v", err)
	}
}
