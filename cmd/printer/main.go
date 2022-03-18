package main

import (
	"flag"
	"fmt"

	_ "github.com/jmacd/caspar.water/cmd/internal"
	"github.com/jmacd/caspar.water/otlp"
	"github.com/jmacd/caspar.water/sparkplug"
	mqtt "github.com/jmacd/caspar.water/sparkplug"
	"google.golang.org/protobuf/encoding/prototext"
)

type state struct {
	otlp.DeviceMap
}

func (s *state) messageReceived(client mqtt.Client, msg mqtt.Message) {
	topic, err := msg.SparkplugBTopic()
	if err != nil {
		fmt.Println("topic parse:", err)
	}

	payload, err := msg.SparkplugBPayload()
	if err != nil {
		fmt.Println("payload parse:", err)
	}

	if topic.MessageType != sparkplug.DDATA && topic.MessageType != sparkplug.DBIRTH {
		fmt.Println("Event", topic, ": ", prototext.Format(payload))
		return
	}

	for _, m := range payload.Metrics {
		id := otlp.SparkplugID{
			GroupID:    topic.GroupID,
			EdgeNodeID: topic.EdgeNodeID,
			DeviceID:   topic.DeviceID,
		}

		var o *otlp.Metric
		if m.GetName() != "" {
			o = s.Define(id, m.GetName(), m.GetAlias(), m.GetTimestamp(), m.GetMetadata().GetDescription())
		} else if m.Alias != nil {
			o = s.Lookup(id, m.GetAlias())
		} else {
			// ERROR! We need a rebirth.
			// @@@
		}
		o.Timestamp = m.GetTimestamp()

		fmt.Println("Metric", o.Name, "=", m.Value)
	}
}

func stateReceived(client mqtt.Client, msg mqtt.Message) {
	fmt.Println("State", msg.Topic(), ": ", string(msg.Payload()))
}

func main() {
	server := flag.String("server", "tcp://localhost:1883", "The MQTT server to connect to")
	flag.Parse()

	sparkTopic := sparkplug.NewTopic("#", "", "", "")

	state := state{otlp.DeviceMap{}}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(*server).SetClientID("printer").SetCleanSession(true)
	opts.SetOnConnectHandler(func(c mqtt.Client) {
		if token := c.SubscribeSparkplug(sparkTopic, 1, state.messageReceived); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
		if token := c.SubscribeString("STATE/+", 1, stateReceived); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
	})

	client := mqtt.NewClient(opts)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	fmt.Printf("Connected to %s\n", *server)

	select {}
}
