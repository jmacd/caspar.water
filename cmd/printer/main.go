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

	fmt.Println(topic, ": ", prototext.Format(payload))
}

func main() {
	server := flag.String("server", "tcp://localhost:1883", "The MQTT server to connect to")
	flag.Parse()

	subTopic := sparkplug.NewTopic("+", sparkplug.DDATA, "+", "+")

	opts := mqtt.NewClientOptions()
	opts.AddBroker(*server).SetClientID("printer").SetCleanSession(true)
	opts.SetOnConnectHandler(func(c mqtt.Client) {
		if token := c.Subscribe(subTopic, 1, messageReceived); token.Wait() && token.Error() != nil {
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
