package sparkplugreceiver

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

const (
	nodeControlPrefix    = "Node Control/"
	nodePropertiesPrefix = "Node Properties/"

	deviceControlPrefix    = "Device Control/"
	devicePropertiesPrefix = "Device Properties/"

	libraryName = "OptoMMP/Modules/Channels"
)

var (
	// Hacky denylist
	denyNames = map[string]bool{
		"Device Properties/Tag Access Time Ms": true,
		"Device Properties/Write Queue Depth":  true,
	}
)

type sparkplugReceiver struct {
	settings     component.ReceiverCreateSettings
	config       Config
	nextConsumer consumer.Metrics
	broker       *mqtt.Server
	brokerDone   chan error
	state        otlp.SparkplugState
	startTime    time.Time
}

var (
	// ErrUnexpectedTopic happens in self-hosted mode where we
	// do not expect another broker or another host application
	// to be using MQTT.
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
		state:        otlp.SparkplugState{}.Init(),
		startTime:    time.Now(),
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
			"server error",
			zap.String("client_id", cl.ID),
			zap.String("remote_addr", cl.Remote),
			zap.Error(err),
		)
	}

	r.broker.Events.OnMessage = func(cl events.Client, pk events.Packet) (events.Packet, error) {
		pk, err := r.onMessage(cl, pk)

		if err != nil {
			r.settings.Logger.Warn(
				"message error",
				zap.String("client_id", cl.ID),
				zap.String("remote_addr", cl.Remote),
				zap.Error(err),
			)
		}

		return pk, err
	}

	return nil
}

func (r *sparkplugReceiver) onMessage(cl events.Client, pk events.Packet) (events.Packet, error) {
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
		return pk, fmt.Errorf("payload unmarshal: %v: %w", pk.TopicName, err)
	}

	return pk, r.sparkplugPayload(topic, b)
}

func (r *sparkplugReceiver) sparkplugPayload(topic sparkplug.Topic, payload *bproto.Payload) error {

	if topic.MessageType == sparkplug.NDATA || topic.MessageType == sparkplug.NBIRTH {
		//fmt.Println("NODE", prototext.Format(payload))
		return r.sparkplugNodePayload(topic, payload)
	}

	if topic.MessageType == sparkplug.DDATA || topic.MessageType == sparkplug.DBIRTH {
		//fmt.Println("DEVICE", prototext.Format(payload))
		return r.sparkplugDevicePayload(topic, payload)
	}

	// Unexpected in a self-hosted broker situation.  STATE/* was
	// checked above, so these are death messages or node/device commands.
	return fmt.Errorf("%w: %v", ErrUnexpectedTopic, topic.MessageType)
}

func (r *sparkplugReceiver) sparkplugNodePayload(topic sparkplug.Topic, payload *bproto.Payload) error {
	node := r.state.Get(topic.GroupID).Get(topic.EdgeNodeID)
	if err := node.Visit(payload); err != nil {
		return err
	}
	metrics := r.nodeToResource(topic, node)
	rm := metrics.ResourceMetrics().At(0)

	ilm := rm.InstrumentationLibraryMetrics().AppendEmpty()
	ilm.InstrumentationLibrary().SetName("discovery")

	metric := ilm.Metrics().AppendEmpty()
	metric.SetName("alive")
	metric.SetDataType(pdata.MetricDataTypeSum)
	metric.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
	metric.Sum().SetIsMonotonic(false)

	dp := metric.Sum().DataPoints().AppendEmpty()
	dp.SetIntVal(1)

	if bd, ok := node.Store.NameMap["bdSeq"]; ok {
		// Note: we do not know the true start timestamp.
		// However, we have no counters for which it matters.
		dp.SetTimestamp(pdata.Timestamp(bd.Timestamp * 1e6))
		dp.SetStartTimestamp(pdata.Timestamp(r.startTime.UnixNano()))
	}

	return r.nextConsumer.ConsumeMetrics(context.Background(), metrics)
}

func (r *sparkplugReceiver) nodeToResource(topic sparkplug.Topic, node otlp.EdgeNodeState) pdata.Metrics {
	m := pdata.NewMetrics()
	rm := m.ResourceMetrics().AppendEmpty()

	rm.Resource().Attributes().InsertString(
		"group_id",
		string(topic.GroupID),
	)
	rm.Resource().Attributes().InsertString(
		"edgenode_id",
		string(topic.EdgeNodeID),
	)

	for _, metric := range node.Store.NameMap {
		switch {

		case metric.Name == "bdSeq":
			continue

		case strings.HasPrefix(metric.Name, nodeControlPrefix):
			continue

		case strings.HasPrefix(metric.Name, nodePropertiesPrefix):
			rm.Resource().Attributes().Insert(
				resourceName(metric.Name[len(nodePropertiesPrefix):]),
				anyValue(metric.Value),
			)
			continue
		}

		r.settings.Logger.Warn(
			"unexpected edge node metric",
			zap.String("name", metric.Name),
		)
	}
	return m
}

func resourceName(name string) string {
	return strings.Replace(strings.ToLower(name), " ", "_", -1)
}

func metricName(name string) string {
	return strings.Replace(strings.ToLower(name), "/", "_", -1)
}

func anyValue(value interface{}) pdata.AttributeValue {
	switch t := value.(type) {
	case *bproto.Payload_Metric_IntValue:
		return pdata.NewAttributeValueInt(int64(t.IntValue))
	case *bproto.Payload_Metric_LongValue:
		return pdata.NewAttributeValueInt(int64(t.LongValue))
	case *bproto.Payload_Metric_FloatValue:
		return pdata.NewAttributeValueDouble(float64(t.FloatValue))
	case *bproto.Payload_Metric_DoubleValue:
		return pdata.NewAttributeValueDouble(t.DoubleValue)
	case *bproto.Payload_Metric_BooleanValue:
		return pdata.NewAttributeValueBool(t.BooleanValue)
	case *bproto.Payload_Metric_StringValue:
		return pdata.NewAttributeValueString(t.StringValue)
	case *bproto.Payload_Metric_BytesValue:
		return pdata.NewAttributeValueString(string(t.BytesValue))

	case *bproto.Payload_Metric_DatasetValue,
		*bproto.Payload_Metric_TemplateValue,
		*bproto.Payload_Metric_ExtensionValue:
		break
	}
	return pdata.NewAttributeValueString(fmt.Sprintf("unsupported attribute type: %T", value))
}

func (r *sparkplugReceiver) floatValue(value interface{}) float64 {
	switch t := value.(type) {
	case *bproto.Payload_Metric_IntValue:
		return float64(t.IntValue)
	case *bproto.Payload_Metric_LongValue:
		return float64(t.LongValue)
	case *bproto.Payload_Metric_FloatValue:
		return float64(t.FloatValue)
	case *bproto.Payload_Metric_DoubleValue:
		return float64(t.DoubleValue)
	}
	r.settings.Logger.Info("non-numeric value",
		zap.String("value", fmt.Sprint(value)),
	)
	return 0
}

func (r *sparkplugReceiver) sparkplugDevicePayload(topic sparkplug.Topic, payload *bproto.Payload) error {
	node := r.state.Get(topic.GroupID).Get(topic.EdgeNodeID)
	device := node.Get(topic.DeviceID)
	if err := device.Visit(payload); err != nil {
		return err
	}
	metrics := r.nodeToResource(topic, node)
	rm := metrics.ResourceMetrics().At(0)

	rm.Resource().Attributes().InsertString(
		"device_id",
		string(topic.DeviceID),
	)

	ilm := rm.InstrumentationLibraryMetrics().AppendEmpty()

	// Hacky hard-coded library name
	ilm.InstrumentationLibrary().SetName(libraryName)

	for _, metric := range device.Store.NameMap {

		if denyNames[metric.Name] {
			continue
		}

		if strings.HasPrefix(metric.Name, deviceControlPrefix) {
			continue
		}

		if strings.HasPrefix(metric.Name, devicePropertiesPrefix) {
			rm.Resource().Attributes().Insert(
				resourceName(metric.Name[len(devicePropertiesPrefix):]),
				anyValue(metric.Value),
			)
			continue
		}

		if !metric.Changed {
			continue
		}
		metric.Changed = false

		name := metric.Name
		if strings.HasPrefix(name, libraryName) {
			name = name[len(libraryName)+1:]
		}

		output := ilm.Metrics().AppendEmpty()
		output.SetName(metricName(name))
		output.SetDataType(pdata.MetricDataTypeGauge)

		dp := output.Gauge().DataPoints().AppendEmpty()
		dp.SetDoubleVal(r.floatValue(metric.Value))

		dp.SetTimestamp(pdata.Timestamp(metric.Timestamp * 1e6))
		dp.SetStartTimestamp(pdata.Timestamp(r.startTime.UnixNano()))
	}

	return r.nextConsumer.ConsumeMetrics(context.Background(), metrics)
}
