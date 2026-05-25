package api

import "encoding/json"

// EventType identifies the kind of event in an SSE stream from the daemon.
type EventType string

// Event type names. Stable strings; clients dispatch on these.
const (
	EventRouteAdded   EventType = "route.added"
	EventRouteChanged EventType = "route.changed"
	EventRouteRemoved EventType = "route.removed"
	EventHealthChange EventType = "health.changed"
	EventStatsTick    EventType = "stats.tick"
	EventLogLine      EventType = "log.line"
)

// Event is the envelope written to the SSE stream. Data is the marshaled
// payload for the given Type; consumers decode it based on Type.
type Event struct {
	Type EventType       `json:"type"`
	Data json.RawMessage `json:"data"`
}

// NewEvent constructs an Event by marshaling payload as Data.
// Returns an Event with empty Data if marshaling fails (callers should never
// pass un-marshalable data; the function is convenience).
func NewEvent(t EventType, payload any) Event {
	b, err := json.Marshal(payload)
	if err != nil {
		return Event{Type: t, Data: json.RawMessage(`null`)}
	}
	return Event{Type: t, Data: b}
}
