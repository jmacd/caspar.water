#
# -ldflags="-s -w" strips DWARF debug info and the symbol table; -trimpath
# removes local filesystem paths from the binary.  Together these typically
# cut the OTel collector binary by ~30-40% which matters a lot when
# uploading to a BeagleBone over a slow link.
GOBUILD := go build -trimpath -ldflags="-s -w"

all:
	@mkdir -p ./build
	go install go.opentelemetry.io/collector/cmd/builder@v0.153.0
	${HOME}/go/bin/builder --config build.yaml --skip-compilation
	(cd build && GOOS=linux GOARCH=arm GOARM=7 $(GOBUILD) -o ../collector.bbb .)
	(cd build && GOOS=linux GOARCH=amd64 $(GOBUILD) -o ../collector.linux .)
	(cd build && $(GOBUILD) -o ../collector.local .)
	@rm -rf ./build
