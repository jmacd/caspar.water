module github.com/jmacd/caspar.water

go 1.17

require (
	github.com/eclipse/paho.mqtt.golang v1.3.5
	github.com/logrusorgru/aurora v2.0.3+incompatible
	github.com/mochi-co/mqtt v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/proto/otlp v0.12.0
	google.golang.org/protobuf v1.27.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rs/xid v1.3.0 // indirect
	golang.org/x/net v0.0.0-20201021035429-f5854403a974 // indirect
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c // indirect
	gopkg.in/yaml.v3 v3.0.0-20200313102051-9f266ea9e77c // indirect
)

replace github.com/mochi-co/mqtt => ../mochi-co-mqtt-jmacd

replace github.com/eclipse/paho.mqtt.golang => ../paho.mqtt.golang-jmacd
