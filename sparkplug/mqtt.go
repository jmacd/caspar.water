package sparkplug

import (
	paho "github.com/eclipse/paho.mqtt.golang"
)

type (
	Client struct {
		client paho.Client
	}

	Message struct {
		message paho.Message
	}
)

func NewClientOptions() *paho.ClientOptions {
	return paho.NewClientOptions()
}

func NewClient(opts *paho.ClientOptions) Client {
	return Client{
		paho.NewClient(opts),
	}
}

func (m Message) Topic() Topic {
	return Topic{
		// @@@
	}
}
