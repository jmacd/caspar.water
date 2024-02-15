#
all:
	@mkdir -p ./build
	go install go.opentelemetry.io/collector/cmd/builder@v0.94.1
	${HOME}/go/bin/builder --config build.yaml
	cd build && GOOS=linux GOARCH=arm GOARM=7 go build -o ../collector.bbb .
	cd build && GOOS=linux GOARCH=amd64 go build -o ../collector.linux .
	cd build && go build -o ../collector.local .
	@rm -rf ./build
