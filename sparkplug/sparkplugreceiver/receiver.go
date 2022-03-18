package sparkplugreceiver

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmacd/caspar.water/otlp"
	"github.com/jmacd/caspar.water/sparkplug"
	"github.com/jmacd/caspar.water/sparkplug/bproto"
	mqtt "github.com/mochi-co/mqtt/server"
	"github.com/mochi-co/mqtt/server/events"
	"github.com/mochi-co/mqtt/server/listeners"
	"github.com/mochi-co/mqtt/server/listeners/auth"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
)

type sparkplugReceiver struct {
	settings     component.ReceiverCreateSettings
	config       Config
	nextConsumer consumer.Metrics
	broker       *mqtt.Server
	brokerDone   chan error
	deviceState  otlp.DeviceMap
}

var (
	ErrUnexpectedTopic = fmt.Errorf("unexpected topic")
)

// New creates the Sparkplug receiver with the given parameters.
func New(
	set component.ReceiverCreateSettings,
	config Config,
	nextConsumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	if nextConsumer == nil {
		return nil, componenterror.ErrNilNextConsumer
	}

	if config.Broker.NetAddr.Endpoint == "" {
		config.Broker.NetAddr.Transport = "tcp"
		config.Broker.NetAddr.Endpoint = "localhost:1883"
	}

	r := &sparkplugReceiver{
		settings:     set,
		config:       config,
		nextConsumer: nextConsumer,
	}
	return r, nil
}

func (r *sparkplugReceiver) Start(ctx context.Context, host component.Host) error {
	if !r.config.Broker.SelfHosted {
		return fmt.Errorf("not implemented: passive client mode")
	}

	if err := r.startBroker(ctx); err != nil {
		return err
	}
	r.settings.Logger.Info(
		"self-hosted broker start",
		zap.String("host_id", r.config.Broker.HostID),
		zap.String("endpoint", r.config.Broker.NetAddr.Endpoint),
	)
	return nil
}

func (r *sparkplugReceiver) Shutdown(context.Context) error {
	_ = r.broker.Close()
	return <-r.brokerDone
}

func (r *sparkplugReceiver) startBroker(context.Context) error {
	r.broker = mqtt.New()
	r.brokerDone = make(chan error)

	switch r.config.Broker.NetAddr.Transport {
	case "tcp":
		break
	default:
		return fmt.Errorf("transport unsupported: %v",
			r.config.Broker.NetAddr.Transport)
	}

	tcp := listeners.NewTCP(
		r.config.Broker.NetAddr.Endpoint,
		r.config.Broker.NetAddr.Endpoint,
	)
	if err := r.broker.AddListener(tcp, &listeners.Config{
		Auth: new(auth.Allow),
	}); err != nil {
		return err
	}

	// Publish the application state with retain=true.
	r.broker.Publish("STATE/"+r.config.Broker.HostID, []byte("ONLINE"), true)

	go func() {
		r.settings.Logger.Info(
			"listening",
			zap.String("endpoint", r.config.Broker.NetAddr.Endpoint),
		)

		r.brokerDone <- r.broker.Serve()
	}()

	r.broker.Events.OnConnect = func(cl events.Client, pk events.Packet) {
		r.settings.Logger.Info(
			"client connected",
			zap.String("client_id", cl.ID),
			zap.String("remote_addr", cl.Remote),
		)
	}

	r.broker.Events.OnDisconnect = func(cl events.Client, err error) {
		r.settings.Logger.Warn(
			"client disconnected",
			zap.String("client_id", cl.ID),
			zap.String("remote_addr", cl.Remote),
			zap.Error(err),
		)
	}

	r.broker.Events.OnError = func(cl events.Client, err error) {
		r.settings.Logger.Error(
			"client error",
			zap.String("client_id", cl.ID),
			zap.String("remote_addr", cl.Remote),
			zap.Error(err),
		)
	}

	r.broker.Events.OnMessage = func(cl events.Client, pk events.Packet) (events.Packet, error) {
		if !strings.HasPrefix(pk.TopicName, sparkplug.BTopicPrefix) {
			// A "STATE/host_id" message is valid but
			// unexpected.  In a self-hosted broker it's not
			// clear who would do this or why.
			return pk, fmt.Errorf("%w: %s", ErrUnexpectedTopic, pk.TopicName)
		}

		topic, err := sparkplug.ParseTopic(pk.TopicName)
		if err != nil {
			return pk, fmt.Errorf("parse topic: %w: %s", err, pk.TopicName)
		}

		b := &bproto.Payload{}
		if err := proto.Unmarshal(pk.Payload, b); err != nil {
			return pk, fmt.Errorf("sparkplug payload: %w", err)
		}

		r.sparkplugPayload(topic, b)

		return pk, nil
	}

	return nil
}

func (r *sparkplugReceiver) sparkplugPayload(topic sparkplug.Topic, payload *bproto.Payload) {

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
			o = r.deviceState.Define(id, m.GetName(), m.GetAlias(), m.GetTimestamp(), m.GetMetadata().GetDescription())
		} else if m.Alias != nil {
			o = r.deviceState.Lookup(id, m.GetAlias())
		} else {
			// ERROR! We need a rebirth.
			// @@@
		}
		o.Timestamp = m.GetTimestamp()

		fmt.Println("Metric", o.Name, "=", m.Value)
	}
}
