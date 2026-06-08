module github.com/jmacd/caspar.water

go 1.24.0

require (
	github.com/Rhymond/go-money v1.0.10
	github.com/influxdata/line-protocol/v2 v2.2.1
	github.com/jmacd/maroto v0.0.0-20230617070925-955e5cabca9e
	github.com/prometheus-community/pro-bing v0.4.0
	github.com/simonvetter/modbus v1.6.3
	github.com/spf13/afero v1.10.0
	github.com/spf13/cobra v1.10.2
	github.com/stretchr/testify v1.11.1
	go.bug.st/serial v1.6.4
	go.opentelemetry.io/collector/component v1.49.0
	go.opentelemetry.io/collector/config/confighttp v0.143.0
	go.opentelemetry.io/collector/config/configopaque v1.49.0
	go.opentelemetry.io/collector/config/configoptional v1.49.0
	go.opentelemetry.io/collector/config/configretry v1.49.0
	go.opentelemetry.io/collector/consumer v1.49.0
	go.opentelemetry.io/collector/consumer/consumererror v0.143.0
	go.opentelemetry.io/collector/exporter v1.49.0
	go.opentelemetry.io/collector/exporter/exporterhelper v0.143.0
	go.opentelemetry.io/collector/pdata v1.49.0
	go.opentelemetry.io/collector/processor v1.49.0
	go.opentelemetry.io/collector/processor/processorhelper v0.143.0
	go.opentelemetry.io/collector/receiver v1.49.0
	go.opentelemetry.io/otel v1.39.0
	go.opentelemetry.io/proto/otlp v1.7.1
	go.uber.org/mock v0.6.0
	go.uber.org/zap v1.27.1
	golang.org/x/exp v0.0.0-20250808145144-a408d31f581a
	golang.org/x/sync v0.19.0
	golang.org/x/text v0.32.0
	gonum.org/v1/plot v0.15.2
	google.golang.org/protobuf v1.36.11
	gopkg.in/natefinch/lumberjack.v2 v2.2.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	codeberg.org/go-fonts/liberation v0.5.0 // indirect
	codeberg.org/go-latex/latex v0.1.0 // indirect
	codeberg.org/go-pdf/fpdf v0.10.0 // indirect
	git.sr.ht/~sbinet/gg v0.6.0 // indirect
	github.com/ajstarks/svgo v0.0.0-20211024235047-1546f124cd8b // indirect
	github.com/boombuler/barcode v1.0.1 // indirect
	github.com/campoy/embedmd v1.0.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/creack/goselect v0.1.2 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20250903184740-5d135037bd4d // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/goburrow/serial v0.1.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/go-tpm v0.9.8 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.27.2 // indirect
	github.com/hashicorp/go-version v1.8.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/jung-kurt/gofpdf v1.16.2 // indirect
	github.com/klauspost/compress v1.18.2 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/pierrec/lz4/v4 v4.1.23 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/ruudk/golang-pdf417 v0.0.0-20201230142125-a7e3863a1245 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/client v1.49.0 // indirect
	go.opentelemetry.io/collector/config/configauth v1.49.0 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.49.0 // indirect
	go.opentelemetry.io/collector/config/configmiddleware v1.49.0 // indirect
	go.opentelemetry.io/collector/config/configtls v1.49.0 // indirect
	go.opentelemetry.io/collector/confmap v1.49.0 // indirect
	go.opentelemetry.io/collector/confmap/xconfmap v0.143.0 // indirect
	go.opentelemetry.io/collector/extension v1.49.0 // indirect
	go.opentelemetry.io/collector/extension/extensionauth v1.49.0 // indirect
	go.opentelemetry.io/collector/extension/extensionmiddleware v0.143.0 // indirect
	go.opentelemetry.io/collector/extension/xextension v0.143.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.49.0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.143.0 // indirect
	go.opentelemetry.io/collector/pdata/xpdata v0.143.0 // indirect
	go.opentelemetry.io/collector/pipeline v1.49.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.64.0 // indirect
	go.opentelemetry.io/otel/metric v1.39.0 // indirect
	go.opentelemetry.io/otel/trace v1.39.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.46.0 // indirect
	golang.org/x/image v0.25.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20251029180050-ab9386a59fda // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251029180050-ab9386a59fda // indirect
	google.golang.org/grpc v1.78.0 // indirect
)

//replace github.com/jmacd/caspar.water/measure/bme280 => ./measure/bme280
