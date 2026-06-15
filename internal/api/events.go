package api

import "encoding/json"

type EventType string

const (
	EventRouteAdded   EventType = "route.added"
	EventRouteChanged EventType = "route.changed"
	EventRouteRemoved EventType = "route.removed"
	EventHealthChange EventType = "health.changed"
	EventStatsTick    EventType = "stats.tick"
	EventStatusTick   EventType = "status.tick"
	EventLogLine      EventType = "log.line"
)

type Event struct {
	Type EventType       `json:"type"`
	Data json.RawMessage `json:"data"`
}

func NewEvent(t EventType, payload any) Event {
	b, err := json.Marshal(payload)
	if err != nil {
		return Event{Type: t, Data: json.RawMessage(`null`)}
	}
	return Event{Type: t, Data: b}
}
