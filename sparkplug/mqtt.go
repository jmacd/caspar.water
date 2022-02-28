package sparkplug

import (
	mqtt "github.com/eclipse/paho.mqtt.golang"
	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/jmacd/caspar.water/sparkplug/bproto"
	"google.golang.org/protobuf/proto"
)

type (
	Client struct {
		client paho.Client
	}

	ClientOptions struct {
		paho.ClientOptions
	}

	Message struct {
		paho.Message
	}
)

func NewClientOptions() *ClientOptions {
	return &ClientOptions{
		ClientOptions: *paho.NewClientOptions(),
	}
}

func NewClient(opts *ClientOptions) Client {
	return Client{
		paho.NewClient(&opts.ClientOptions),
	}
}

func (m Message) SparkplugBTopic() (Topic, error) {
	return ParseTopic(m.Message.Topic())
}

func (m Message) SparkplugBPayload() (*bproto.Payload, error) {
	b := &bproto.Payload{}
	return b, proto.Unmarshal(m.Message.Payload(), b)
}

type OnConnectHandler func(Client)

func (o *ClientOptions) SetOnConnectHandler(onConn OnConnectHandler) *ClientOptions {
	o.ClientOptions.SetOnConnectHandler(func(pc paho.Client) {
		onConn(Client{client: pc})
	})
	return o
}

type MessageHandler func(Client, Message)

func (c Client) Subscribe(topic Topic, qos byte, callback MessageHandler) mqtt.Token {
	return c.client.Subscribe(topic.String(), qos, func(c mqtt.Client, m mqtt.Message) {
		callback(Client{
			client: c,
		}, Message{
			Message: m,
		})
	})
}

func (c Client) Connect() mqtt.Token {
	return c.client.Connect()
}

func (c Client) Publish(topic Topic, qos byte, retained bool, payload *bproto.Payload) mqtt.Token {
	data, err := proto.Marshal(payload)
	if err != nil {
		// @@@
		panic(err)
	}
	return c.client.Publish(topic.String(), qos, retained, data)
}
