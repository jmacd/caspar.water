// Copyright ...
//
//go:generate stringer -type=MessageType
package sparkplug

import (
	"errors"
	"strings"

	"github.com/jmacd/caspar.water/sparkplug/bproto"
)

type (
	GroupID    string
	EdgeNodeID string
	DeviceID   string

	Topic struct {
		GroupID     GroupID
		MessageType MessageType
		EdgeNodeID  EdgeNodeID
		DeviceID    DeviceID
	}

	MessageType string

	Payload = bproto.Payload
)

const (
	BTopicPrefix = "spBv1.0"

	NBIRTH MessageType = "NBIRTH" // Birth certificate for MQTT EoN nodes.
	NDEATH MessageType = "NDEATH" // Death certificate for MQTT EoN nodes.
	DBIRTH MessageType = "DBIRTH" // Birth certificate for Devices.
	DDEATH MessageType = "DDEATH" // Death certificate for Devices.
	NDATA  MessageType = "NDATA"  // Node data message.
	DDATA  MessageType = "DDATA"  // Device data message.
	NCMD   MessageType = "NCMD"   // Node command message.
	DCMD   MessageType = "DCMD"   // Device command message.
	STATE  MessageType = "STATE"  // Critical application state message.
	ANY    MessageType = "+"
)

var (
	ErrTopicSyntax        = errors.New("invalid topic syntax")
	ErrSparkplugNamespace = errors.New("invalid sparkplug namespace")
	ErrInvalidMessageType = errors.New("invalid sparkplug message type")
)

func (t Topic) String() string {
	a := []string{
		BTopicPrefix,
		string(t.GroupID),
		string(t.MessageType),
		string(t.EdgeNodeID),
	}
	if t.DeviceID != "" {
		a = append(a, string(t.DeviceID))
	}
	for i, x := range a {
		if x == "#" {
			a = a[:i+1]
			break
		}
	}
	return strings.Join(a, "/")
}

func ParseTopic(ts string) (Topic, error) {
	elems := strings.Split(ts, "/")

	if len(elems) < 4 || len(elems) > 5 {
		return Topic{}, ErrTopicSyntax
	}

	if elems[0] != BTopicPrefix {
		return Topic{}, ErrSparkplugNamespace
	}

	for _, s := range elems[1:4] {
		if s == "" {
			return Topic{}, ErrTopicSyntax
		}
	}

	switch MessageType(elems[2]) {
	case NBIRTH, NDEATH, DBIRTH, DDEATH, NDATA, DDATA, NCMD, DCMD, STATE:
	default:
		return Topic{}, ErrInvalidMessageType
	}

	var device string
	if len(elems) == 5 {
		device = elems[4]
	}

	return Topic{
		GroupID:     GroupID(elems[1]),
		MessageType: MessageType(elems[2]),
		EdgeNodeID:  EdgeNodeID(elems[3]),
		DeviceID:    DeviceID(device),
	}, nil
}

func NewTopic(grp GroupID, mt MessageType, edge EdgeNodeID, dev DeviceID) Topic {
	return Topic{
		GroupID:     grp,
		MessageType: mt,
		EdgeNodeID:  edge,
		DeviceID:    dev,
	}
}
