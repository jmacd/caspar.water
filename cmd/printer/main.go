package main

import (
	"flag"
	"fmt"

	_ "github.com/jmacd/caspar.water/cmd/internal"
	"github.com/jmacd/caspar.water/otlp"
	"github.com/jmacd/caspar.water/sparkplug"
	"github.com/jmacd/caspar.water/sparkplug/sparkplugclient"
	"google.golang.org/protobuf/encoding/prototext"
)

type state struct {
	otlp.SparkplugState
}

func (s *state) messageReceived(client sparkplugclient.Client, msg sparkplugclient.Message) {
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

	node := s.Get(topic.GroupID).Get(topic.EdgeNodeID)
	device := node.Get(topic.DeviceID)
	if err := device.Visit(topic, payload); err != nil {
		fmt.Println("ERROR", topic, ":", err)
	}

	fmt.Println("Device", device)
}

func stateReceived(client sparkplugclient.Client, msg sparkplugclient.Message) {
	fmt.Println("State", msg.Topic(), ": ", string(msg.Payload()))
}

func main() {
	server := flag.String("server", "tcp://localhost:1883", "The MQTT server to connect to")
	flag.Parse()

	sparkTopic := sparkplug.NewTopic("#", "", "", "")

	state := state{}
	state.SparkplugState = state.SparkplugState.Init()

	opts := sparkplugclient.NewOptions()
	opts.AddBroker(*server).SetClientID("printer").SetCleanSession(true)
	opts.SetOnConnectHandler(func(c sparkplugclient.Client) {
		if token := c.SubscribeSparkplug(sparkTopic, 1, state.messageReceived); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
		if token := c.SubscribeString("STATE/+", 1, stateReceived); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
	})

	client := sparkplugclient.NewClient(opts)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	fmt.Printf("Connected to %s\n", *server)

	select {}
}
