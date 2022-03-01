package main

import (
	"flag"
	"fmt"

	"github.com/jmacd/caspar.water/sparkplug"
	mqtt "github.com/jmacd/caspar.water/sparkplug"
	"google.golang.org/protobuf/encoding/prototext"
)

func messageReceived(client mqtt.Client, msg mqtt.Message) {
	topic, err := msg.SparkplugBTopic()
	if err != nil {
		fmt.Println("topic parse:", err)
	}

	payload, err := msg.SparkplugBPayload()
	if err != nil {
		fmt.Println("payload parse:", err)
	}

	fmt.Println("Event", topic, ": ", prototext.Format(payload))
}

func stateReceived(client mqtt.Client, msg mqtt.Message) {
	fmt.Println("State", msg.Topic(), ": ", string(msg.Payload()))
}

func main() {
	server := flag.String("server", "tcp://localhost:1883", "The MQTT server to connect to")
	flag.Parse()

	sparkTopic := sparkplug.NewTopic("+", sparkplug.ANY, "+", "+")

	opts := mqtt.NewClientOptions()
	opts.AddBroker(*server).SetClientID("printer").SetCleanSession(true)
	opts.SetOnConnectHandler(func(c mqtt.Client) {
		if token := c.SubscribeSparkplug(sparkTopic, 1, messageReceived); token.Wait() && token.Error() != nil {
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
