MODULES := $(shell find . -name go.mod)
GOCMD := go
GODIRS := $(foreach d,$(MODULES),$(shell dirname $d))
WATERTOWN_TARGET_DIR ?= $(CURDIR)/watertown/target

.PHONY: all tidy test-water-config test-analytics-scripts

all:
	for dir in $(GODIRS); do (cd $${dir}; $(GOCMD) test ./...) || exit 1; done
	cd collector && $(MAKE)

tidy:
	for dir in $(GODIRS); do (cd $${dir}; GOWORK=off $(GOCMD) mod tidy) || exit 1; done

test-water-config:
	cd watertown && CARGO_TARGET_DIR="$(WATERTOWN_TARGET_DIR)" cargo build -p cmd
	POND_BIN="$(WATERTOWN_TARGET_DIR)/debug/pond" config/scripts/smoke-water-config.sh

test-analytics-scripts:
	bash -n config/scripts/pond.sh
	bash -n config/scripts/run-analytics-report.sh
	python3 config/scripts/test-analytics-report.py
