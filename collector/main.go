package main

import (
	"log"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/loggingexporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/service"

	"github.com/jmacd/caspar.water/sparkplug/sparkplugreceiver"
	"github.com/jmacd/caspar.water/storage/fileexporter"
)

func main() {
	var err error
	factories := component.Factories{}

	factories.Exporters, err = component.MakeExporterFactoryMap(
		loggingexporter.NewFactory(),
		otlpexporter.NewFactory(),
		fileexporter.NewFactory(),
	)
	if err != nil {
		log.Fatal("could not register exporters", err)
	}

	factories.Receivers, err = component.MakeReceiverFactoryMap(
		sparkplugreceiver.NewFactory(),
	)
	if err != nil {
		log.Fatal("could not register receivers", err)
	}

	info := component.BuildInfo{
		Command:     "caspar-water-collector",
		Description: "Caspar Water OpenTelemetry Collector distribution",
		Version:     "0.1.0",
	}

	settings := service.CollectorSettings{
		BuildInfo: info,
		Factories: factories,
	}

	app := service.NewCommand(settings)
	err = app.Execute()
	if err != nil {
		log.Fatal("application run finished with error", err)
	}
}
