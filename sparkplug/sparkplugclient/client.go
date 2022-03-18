package sparkplugclient

import (
	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/jmacd/caspar.water/sparkplug"
	"github.com/jmacd/caspar.water/sparkplug/bproto"
	"google.golang.org/protobuf/proto"
)

type (
	Message struct {
		paho.Message
	}

	Client struct {
		client paho.Client
	}

	Options struct {
		paho.ClientOptions
	}
)

func (m Message) SparkplugBTopic() (sparkplug.Topic, error) {
	return sparkplug.ParseTopic(m.Message.Topic())
}

func (m Message) SparkplugBPayload() (*bproto.Payload, error) {
	b := &bproto.Payload{}
	return b, proto.Unmarshal(m.Message.Payload(), b)
}

func NewOptions() *Options {
	return &Options{
		ClientOptions: *paho.NewClientOptions(),
	}
}

func NewClient(opts *Options) Client {
	return Client{
		paho.NewClient(&opts.ClientOptions),
	}
}

type OnConnectHandler func(Client)

func (o *Options) SetOnConnectHandler(onConn OnConnectHandler) *Options {
	o.ClientOptions.SetOnConnectHandler(func(pc paho.Client) {
		onConn(Client{client: pc})
	})
	return o
}

type MessageHandler func(Client, Message)

func (c Client) SubscribeSparkplug(topic sparkplug.Topic, qos byte, callback MessageHandler) paho.Token {
	return c.client.Subscribe(topic.String(), qos, func(c paho.Client, m paho.Message) {
		callback(Client{
			client: c,
		}, Message{
			Message: m,
		})
	})
}

func (c Client) SubscribeString(topic string, qos byte, callback MessageHandler) paho.Token {
	return c.client.Subscribe(topic, qos, func(c paho.Client, m paho.Message) {
		callback(Client{
			client: c,
		}, Message{
			Message: m,
		})
	})
}

func (c Client) Connect() paho.Token {
	return c.client.Connect()
}

func (c Client) PublishSparkplug(topic sparkplug.Topic, qos byte, retained bool, payload *bproto.Payload) paho.Token {
	data, err := proto.Marshal(payload)
	if err != nil {
		// @@@
		panic(err)
	}
	return c.client.Publish(topic.String(), qos, retained, data)
}

// func (c Client) PublishString(topic Topic, qos byte, retained bool, payload string) paho.Token {
// 	return c.client.Publish(topic.String(), qos, retained, payload)
// }

func (c Client) PublishString(topic string, qos byte, retained bool, payload string) paho.Token {
	return c.client.Publish(topic, qos, retained, payload)
}
