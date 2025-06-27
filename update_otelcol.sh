#!/bin/bash -e

TO0=v0.128.0
TO1=v1.34.0

COMPS="go.opentelemetry.io/collector/exporter/debugexporter go.opentelemetry.io/collector/exporter/otlpexporter github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otelarrowexporter go.opentelemetry.io/collector/receiver/otlpreceiver github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver go.opentelemetry.io/collector/processor/batchprocessor go.opentelemetry.io/collector/connector/forwardconnector "

#sed -i '' "s/$1/$2/g" ./collector/Makefile
#sed -i '' "s/$1/$2/g" ./collector/build.yaml

for comp in ${COMPS}; do
    go get ${comp}@${TO0}
done

# function goget {
#     local DEPS
#     DEPS=`grep "go.opentelemetry.io/collector.* v1" go.mod | awk '{print $1}'`

#     echo DEPS1 $DEPS
#     for dep in ${DEPS}; do
# 	echo go get ${dep}@${TO1}
# 	go get ${dep}@${TO1}
#     done

#     DEPS=`grep "go.opentelemetry.io/collector.* v0" go.mod | awk '{print $1}'`

#     echo DEPS0 $DEPS
#     for dep in ${DEPS}; do
# 	echo go get ${dep}@${TO0}
# 	go get ${dep}@${TO0}
#     done
# }

# (cd . && goget)

(cd collector && make && ./collector.local validate --config test.yaml)
