package main

import (
	"flag"
	"fmt"

	_ "github.com/jmacd/caspar.water/cmd/internal"
	"github.com/jmacd/caspar.water/sparkplug/sparkplugclient"
)

func main() {
	server := flag.String("server", "tcp://localhost:1883", "The MQTT server to connect to")
	flag.Parse()

	stateTopicString := "STATE/waterco"

	opts := sparkplugclient.NewOptions()
	opts.AddBroker(*server).SetClientID("operator").SetCleanSession(false)
	opts.SetWill(stateTopicString, "OFFLINE", 1, true)
	opts.SetOnConnectHandler(func(c sparkplugclient.Client) {
		if token := c.PublishString(stateTopicString, 1, true, "ONLINE"); token.Wait() && token.Error() != nil {
			fmt.Println("subscribe:", token.Error())
		}
	})

	client := sparkplugclient.NewClient(opts)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	fmt.Printf("Connected to %s\n", *server)

	select {}
}
