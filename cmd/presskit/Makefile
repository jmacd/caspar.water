#

all:
	go install go.opentelemetry.io/collector/cmd/builder@latest
	${HOME}/go/bin/builder --config presskit-build.yaml
	cd build && GOOS=linux GOARCH=arm GOARM=7 go build .
	mv ./build/builder ./build/presskit
