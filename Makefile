MODULES := $(shell find . -name go.mod)
GOCMD := go
GODIRS := $(foreach d,$(MODULES),$(shell dirname $d))
WATERTOWN_TARGET_DIR ?= $(CURDIR)/watertown/target

.PHONY: all tidy test-water-config test-beacons

all:
	for dir in $(GODIRS); do (cd $${dir}; $(GOCMD) test ./...) || exit 1; done
	cd collector && $(MAKE)

tidy:
	for dir in $(GODIRS); do (cd $${dir}; GOWORK=off $(GOCMD) mod tidy) || exit 1; done

test-water-config:
	cd watertown && CARGO_TARGET_DIR="$(WATERTOWN_TARGET_DIR)" cargo build -p cmd
	POND_BIN="$(WATERTOWN_TARGET_DIR)/debug/pond" config/scripts/smoke-water-config.sh

test-beacons:
	cd watertown && CARGO_TARGET_DIR="$(WATERTOWN_TARGET_DIR)" cargo build --locked -p cmd --bin pond
	POND_BIN="$(WATERTOWN_TARGET_DIR)/debug/pond" python3 config/scripts/test-beacons.py
	cd watertown/testsuite/browser && node --test tests/analytics.test.mjs
