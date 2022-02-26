package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/jmacd/caspar.water/sparkplug"
	mqtt "github.com/jmacd/caspar.water/sparkplug"
)

func messageReceived(client mqtt.Client, msg mqtt.Message) {
	// topics := strings.Split(msg.Topic(), "/")
	// msgFrom := topics[len(topics)-1]
	fmt.Print(msg.Topic().DeviceID + ": " + string(msg.Payload()))
}

func main() {
	stdin := bufio.NewReader(os.Stdin)
	rand.Seed(time.Now().Unix())

	server := flag.String("server", "tcp://localhost:1883", "The MQTT server to connect to")
	room := flag.String("room", "gochat", "The chat room to enter. default 'gochat'")
	name := flag.String("name", "user"+strconv.Itoa(rand.Intn(1000)), "Username to be displayed")
	flag.Parse()

	subTopic := sparkplug.NewTopic("chat", sparkplug.DDATA, *room, "+")
	pubTopic := sparkplug.NewTopic("chat", sparkplug.DDATA, *room, *name)

	opts := mqtt.NewClientOptions().AddBroker(*server).SetClientID(*name).SetCleanSession(true)

	opts.OnConnect = func(c mqtt.Client) {
		if token := c.Subscribe(subTopic, 1, messageReceived); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
	}

	client := mqtt.NewClient(opts)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	fmt.Printf("Connected as %s to %s\n", *name, *server)

	for {
		message, err := stdin.ReadString('\n')
		if err == io.EOF {
			os.Exit(0)
		}
		if token := client.Publish(pubTopic, 1, false, message); token.Wait() && token.Error() != nil {
			fmt.Println("Failed to send message")
		}
	}
}
