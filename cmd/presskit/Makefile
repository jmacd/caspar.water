#

all:
	go install go.opentelemetry.io/collector/cmd/builder@latest
	${HOME}/go/bin/builder --config presskit-build.yaml
	cd build && go build .
