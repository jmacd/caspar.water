package otlp

import (
	"fmt"
	"time"
	
	otlpcommon "go.opentelemetry.io/proto/otlp/common/v1"

	"github.com/jmacd/caspar.water/sparkplug"
	"github.com/jmacd/caspar.water/sparkplug/bproto"
)

type (
	Initr[T any] interface {
		Init() T
	}

	Items[K comparable, T Initr[T]] map[K]T

	DeviceState struct {
		Store
	}

	EdgeNodeState struct {
		Store
		Items[sparkplug.DeviceID, DeviceState]
	}

	GroupState struct {
		Items[sparkplug.EdgeNodeID, EdgeNodeState]
	}

	SparkplugState struct {
		Items[sparkplug.GroupID, GroupState]
	}

	Store struct {
		AliasMap  AliasMap
		MetricMap MetricMap
	}

	AliasMap map[string]uint64

	MetricMap map[uint64]*Metric

	Metric struct {
		Name           string
		StartTimestamp uint64
		Timestamp      uint64
		Description    string
		Value          otlpcommon.AnyValue
	}
)

var (
	// ErrRebirthNeeded occurs in a passive-listener context if
	// the Sparkplug application reconnects properly and assuming
	// ordered delivery.
	ErrRebirthNeeded = fmt.Errorf("rebirth is needed")
)

func (SparkplugState) Init() SparkplugState {
	return SparkplugState{
		Items: Items[sparkplug.GroupID, GroupState]{},
	}
}

func (GroupState) Init() GroupState {
	return GroupState{
		Items: Items[sparkplug.EdgeNodeID, EdgeNodeState]{},
	}
}

func (EdgeNodeState) Init() EdgeNodeState {
	return EdgeNodeState{
		Items: Items[sparkplug.DeviceID, DeviceState]{},
		Store: Store{}.Init(),
	}
}

func (DeviceState) Init() DeviceState {
	return DeviceState{
		Store: Store{}.Init(),
	}
}

func (Store) Init() Store {
	return Store{
		MetricMap: MetricMap{},
		AliasMap:  AliasMap{},
	}
}

func (items Items[K, T]) Get(id K) T {
	val, ok := items[id]
	if ok {
		return val
	}
	var t T
	t = t.Init()
	items[id] = t
	return t
}

func (st Store) Define(name string, alias, ts uint64, desc string) *Metric {
	if name == "" {
		return st.MetricMap[alias]
	}

	st.AliasMap[name] = alias

	metric, ok := st.MetricMap[alias]

	// Note: this code doesn't check for redefinition using existing aliases.

	if !ok {
		metric = &Metric{
			Name:           name,
			StartTimestamp: ts,
			Description:    desc,
		}
		st.MetricMap[alias] = metric
	}
	return metric
}

func (st Store) Visit(payload *bproto.Payload) error {
	for _, m := range payload.Metrics {

		o := st.Define(m.GetName(), m.GetAlias(), m.GetTimestamp(), m.GetMetadata().GetDescription())
		if o == nil {
			return ErrRebirthNeeded
		}
		o.Update(m.GetTimestamp(), m.GetValue())
	}
	return nil
}

func (m *Metric) Update(ts uint64, value interface{}) {
	fmt.Println("Metric:", m.Name, "=", value, "@", time.UnixMilli(int64(ts)))
}
