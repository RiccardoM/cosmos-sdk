package types

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/gogo/protobuf/jsonpb"
	proto "github.com/gogo/protobuf/proto"
	abci "github.com/tendermint/tendermint/abci/types"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
)

var (
	txEvents = "tm.event='Tx'"
	blEvents = "tm.event='NewBlock'"
)

// ----------------------------------------------------------------------------
// Event Manager
// ----------------------------------------------------------------------------

// EventManager implements a simple wrapper around a slice of Event objects that
// can be emitted from.
type EventManager struct {
	events Events
}

func NewEventManager() *EventManager {
	return &EventManager{EmptyEvents()}
}

func (em *EventManager) Events() Events { return em.events }

// EmitEvent stores a single Event object.
// Deprecated: Use EmitTypedEvent
func (em *EventManager) EmitEvent(event Event) {
	em.events = em.events.AppendEvent(event)
}

// EmitEvents stores a series of Event objects.
// Deprecated: Use EmitTypedEvents
func (em *EventManager) EmitEvents(events Events) {
	em.events = em.events.AppendEvents(events)
}

// ABCIEvents returns all stored Event objects as abci.Event objects.
func (em EventManager) ABCIEvents() []abci.Event {
	return em.events.ToABCIEvents()
}

// EmitTypedEvent takes typed event and emits converting it into Event
func (em *EventManager) EmitTypedEvent(tev proto.Message) error {
	event, err := TypedEventToEvent(tev)
	if err != nil {
		return err
	}

	em.EmitEvent(event)
	return nil
}

// EmitTypedEvents takes series of typed events and emit
func (em *EventManager) EmitTypedEvents(tevs ...proto.Message) error {
	events := make(Events, len(tevs))
	for i, tev := range tevs {
		res, err := TypedEventToEvent(tev)
		if err != nil {
			return err
		}
		events[i] = res
	}

	em.EmitEvents(events)
	return nil
}

// TypedEventToEvent takes typed event and converts to Event object
func TypedEventToEvent(tev proto.Message) (Event, error) {
	// Replace "." in event type with "-" to fix tm event query issue
	evtType := strings.ReplaceAll(proto.MessageName(tev), ".", "-")
	evtJSON, err := codec.ProtoMarshalJSON(tev, nil)
	if err != nil {
		return Event{}, err
	}

	var attrMap map[string]interface{}
	err = json.Unmarshal(evtJSON, &attrMap)
	if err != nil {
		return Event{}, err
	}

	attrs := make([]abci.EventAttribute, 0, len(attrMap))
	for k, v := range attrMap {
		var valueBz []byte
		switch v := v.(type) {
		case string:
			valueBz = []byte(v)
		default:
			var err error
			valueBz, err = json.Marshal(v)
			if err != nil {
				return Event{}, err
			}
		}

		attrs = append(attrs, abci.EventAttribute{
			Key:   []byte(k),
			Value: valueBz,
		})
	}

	return Event{
		Type:       evtType,
		Attributes: attrs,
	}, nil
}

// ParseTypedEvent converts abci.Event back to typed event
func ParseTypedEvent(event abci.Event) (proto.Message, error) {
	// Revert changes in event type name by replacing "-" with "."
	evtType := strings.ReplaceAll(event.Type, "-", ".")
	concreteGoType := proto.MessageType(evtType)
	if concreteGoType == nil {
		return nil, fmt.Errorf("failed to retrieve the message of type %q", event.Type)
	}

	var value reflect.Value
	if concreteGoType.Kind() == reflect.Ptr {
		value = reflect.New(concreteGoType.Elem())
	} else {
		value = reflect.Zero(concreteGoType)
	}

	protoMsg, ok := value.Interface().(proto.Message)
	if !ok {
		return nil, fmt.Errorf("%q does not implement proto.Message", event.Type)
	}

	attrMap := make(map[string]interface{})
	for _, attr := range event.Attributes {
		var value interface{}
		err := json.Unmarshal(attr.Value, &value)
		if err != nil {
			value = string(attr.Value)
		}

		attrMap[string(attr.Key)] = value
	}

	attrBytes, err := json.Marshal(attrMap)
	if err != nil {
		return nil, err
	}

	err = jsonpb.Unmarshal(strings.NewReader(string(attrBytes)), protoMsg)
	if err != nil {
		return nil, err
	}

	return protoMsg, nil
}

// ResultEventToABCIEvent takes the ctypes.ResultEvent and casts it to abci.TxResult, extracting the []abci.Event
func ResultEventToABCIEvent(rev ctypes.ResultEvent) ([]abci.Event, error) {
	switch rev.Query {
	case txEvents:
		var txResult abci.TxResult
		txResBytes, err := json.Marshal(rev.Data)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(txResBytes, &txResult); err != nil {
			return nil, fmt.Errorf("failed to unmarshall into abci.TxResult: %s", string(txResBytes))
		}
		return txResult.Result.Events, nil
	case blEvents:
		var blResult tmtypes.EventDataNewBlock
		bl, err := json.Marshal(rev.Data)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(bl, &blResult); err != nil {
			return nil, fmt.Errorf("failed to unmarshal into tmtypes.EventDataNewBlock: %s", string(bl))
		}
		out := []abci.Event{}
		out = append(out, blResult.ResultBeginBlock.Events...)
		out = append(out, blResult.ResultEndBlock.Events...)
		return out, nil
	default:
		return nil, fmt.Errorf("neither tx nor new block event: %s", rev.Query)
	}
}

// ----------------------------------------------------------------------------
// Events
// ----------------------------------------------------------------------------

type (
	// Event is a type alias for an ABCI Event
	Event abci.Event

	// Events defines a slice of Event objects
	Events []Event
)

// NewEvent creates a new Event object with a given type and slice of one or more
// attributes.
func NewEvent(ty string, attrs ...Attribute) Event {
	e := Event{Type: ty}

	for _, attr := range attrs {
		e.Attributes = append(e.Attributes, attr.ToKVPair())
	}

	return e
}

// NewAttribute returns a new key/value Attribute object.
func NewAttribute(k, v string) Attribute {
	return Attribute{k, v}
}

// EmptyEvents returns an empty slice of events.
func EmptyEvents() Events {
	return make(Events, 0)
}

func (a Attribute) String() string {
	return fmt.Sprintf("%s: %s", a.Key, a.Value)
}

// ToKVPair converts an Attribute object into a Tendermint key/value pair.
func (a Attribute) ToKVPair() abci.EventAttribute {
	return abci.EventAttribute{Key: toBytes(a.Key), Value: toBytes(a.Value)}
}

// AppendAttributes adds one or more attributes to an Event.
func (e Event) AppendAttributes(attrs ...Attribute) Event {
	for _, attr := range attrs {
		e.Attributes = append(e.Attributes, attr.ToKVPair())
	}
	return e
}

// AppendEvent adds an Event to a slice of events.
func (e Events) AppendEvent(event Event) Events {
	return append(e, event)
}

// AppendEvents adds a slice of Event objects to an exist slice of Event objects.
func (e Events) AppendEvents(events Events) Events {
	return append(e, events...)
}

// ToABCIEvents converts a slice of Event objects to a slice of abci.Event
// objects.
func (e Events) ToABCIEvents() []abci.Event {
	res := make([]abci.Event, len(e))
	for i, ev := range e {
		res[i] = abci.Event{Type: ev.Type, Attributes: ev.Attributes}
	}

	return res
}

func toBytes(i interface{}) []byte {
	switch x := i.(type) {
	case []uint8:
		return x
	case string:
		return []byte(x)
	default:
		panic(i)
	}
}

// Common event types and attribute keys
var (
	EventTypeMessage = "message"

	AttributeKeyAction = "action"
	AttributeKeyModule = "module"
	AttributeKeySender = "sender"
	AttributeKeyAmount = "amount"
)

type (
	// StringAttributes defines a slice of StringEvents objects.
	StringEvents []StringEvent
)

func (se StringEvents) String() string {
	var sb strings.Builder

	for _, e := range se {
		sb.WriteString(fmt.Sprintf("\t\t- %s\n", e.Type))

		for _, attr := range e.Attributes {
			sb.WriteString(fmt.Sprintf("\t\t\t- %s\n", attr.String()))
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// Flatten returns a flattened version of StringEvents by grouping all attributes
// per unique event type.
func (se StringEvents) Flatten() StringEvents {
	flatEvents := make(map[string][]Attribute)

	for _, e := range se {
		flatEvents[e.Type] = append(flatEvents[e.Type], e.Attributes...)
	}
	keys := make([]string, 0, len(flatEvents))
	res := make(StringEvents, 0, len(flatEvents)) // appeneded to keys, same length of what is allocated to keys

	for ty := range flatEvents {
		keys = append(keys, ty)
	}

	sort.Strings(keys)
	for _, ty := range keys {
		res = append(res, StringEvent{Type: ty, Attributes: flatEvents[ty]})
	}

	return res
}

// StringifyEvent converts an Event object to a StringEvent object.
func StringifyEvent(e abci.Event) StringEvent {
	res := StringEvent{Type: e.Type}

	for _, attr := range e.Attributes {
		res.Attributes = append(
			res.Attributes,
			Attribute{string(attr.Key), string(attr.Value)},
		)
	}

	return res
}

// StringifyEvents converts a slice of Event objects into a slice of StringEvent
// objects.
func StringifyEvents(events []abci.Event) StringEvents {
	res := make(StringEvents, 0, len(events))

	for _, e := range events {
		res = append(res, StringifyEvent(e))
	}

	return res.Flatten()
}

// MarkEventsToIndex returns the set of ABCI events, where each event's attribute
// has it's index value marked based on the provided set of events to index.
func MarkEventsToIndex(events []abci.Event, indexSet map[string]struct{}) []abci.Event {
	indexAll := len(indexSet) == 0
	updatedEvents := make([]abci.Event, len(events))

	for i, e := range events {
		updatedEvent := abci.Event{
			Type:       e.Type,
			Attributes: make([]abci.EventAttribute, len(e.Attributes)),
		}

		for j, attr := range e.Attributes {
			_, index := indexSet[fmt.Sprintf("%s.%s", e.Type, attr.Key)]
			updatedAttr := abci.EventAttribute{
				Key:   attr.Key,
				Value: attr.Value,
				Index: index || indexAll,
			}

			updatedEvent.Attributes[j] = updatedAttr
		}

		updatedEvents[i] = updatedEvent
	}

	return updatedEvents
}
