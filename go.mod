module github.com/jmacd/caspar.water

go 1.18

require (
	github.com/Rhymond/go-money v1.0.9
	github.com/eclipse/paho.mqtt.golang v1.3.5
	github.com/jmacd/maroto v0.0.0-00010101000000-000000000000
	github.com/logrusorgru/aurora v2.0.3+incompatible
	github.com/mochi-co/mqtt v1.2.3
	github.com/stretchr/testify v1.8.1
	go.opentelemetry.io/collector v0.72.0
	go.opentelemetry.io/collector/component v0.72.0
	go.opentelemetry.io/collector/consumer v0.72.0
	go.opentelemetry.io/collector/pdata v1.0.0-rc6
	go.opentelemetry.io/proto/otlp v0.18.0
	go.uber.org/zap v1.24.0
	google.golang.org/protobuf v1.28.1
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
)

require (
	github.com/boombuler/barcode v1.0.1 // indirect
	github.com/cenkalti/backoff/v4 v4.2.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.7.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/jung-kurt/gofpdf v1.16.2 // indirect
	github.com/knadh/koanf v1.5.0 // indirect
	github.com/kr/pretty v0.3.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rs/xid v1.4.0 // indirect
	github.com/ruudk/golang-pdf417 v0.0.0-20201230142125-a7e3863a1245 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/collector/confmap v0.72.0 // indirect
	go.opentelemetry.io/collector/featuregate v0.72.0 // indirect
	go.opentelemetry.io/otel v1.13.0 // indirect
	go.opentelemetry.io/otel/metric v0.36.0 // indirect
	go.opentelemetry.io/otel/trace v1.13.0 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.9.0 // indirect
	golang.org/x/net v0.7.0 // indirect
	golang.org/x/sys v0.5.0 // indirect
	golang.org/x/text v0.7.0 // indirect
	google.golang.org/genproto v0.0.0-20230110181048-76db0878b65f // indirect
	google.golang.org/grpc v1.53.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

//replace github.com/mochi-co/mqtt => ../mochi-co-mqtt-jmacd

// replace github.com/eclipse/paho.mqtt.golang => ../paho.mqtt.golang-jmacd

// replace github.com/Rhymond/go-money => ../go-money

replace github.com/jmacd/maroto => ../maroto
