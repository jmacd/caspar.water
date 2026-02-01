MODULES := $(shell find . -name go.mod)
GOCMD := go
GODIRS := $(foreach d,$(MODULES),$(shell dirname $d))

.PHONY: all tidy

all:
	for dir in $(GODIRS); do (cd $${dir}; $(GOCMD) test ./...) || exit 1; done
	cd collector && $(MAKE)

tidy:
	for dir in $(GODIRS); do (cd $${dir}; GOWORK=off $(GOCMD) mod tidy) || exit 1; done
