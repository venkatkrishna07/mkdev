package daemon

import (
	"log/slog"
	"sync"

	"github.com/venkatkrishna07/mkdev/internal/api"
)

// hubChannelCap bounds each subscriber's buffer. On overflow the hub drops the
// oldest event for that subscriber and logs a warning.
const hubChannelCap = 256

// Hub fans out api.Events to active subscribers. Safe for concurrent use.
// A subscriber receives only events published after Subscribe returns.
type Hub struct {
	mu     sync.Mutex
	nextID int
	subs   map[int]chan api.Event
	closed bool
}

// NewHub returns an empty Hub.
func NewHub() *Hub {
	return &Hub{subs: map[int]chan api.Event{}}
}

// Subscribe registers a new subscriber and returns its ID + read-only channel.
// Caller must invoke Unsubscribe(id) to release the channel.
func (h *Hub) Subscribe() (int, <-chan api.Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		ch := make(chan api.Event)
		close(ch)
		return -1, ch
	}
	h.nextID++
	id := h.nextID
	ch := make(chan api.Event, hubChannelCap)
	h.subs[id] = ch
	return id, ch
}

// Unsubscribe removes the subscriber and closes its channel.
func (h *Hub) Unsubscribe(id int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	ch, ok := h.subs[id]
	if !ok {
		return
	}
	delete(h.subs, id)
	close(ch)
}

// Publish broadcasts an event to all current subscribers. Non-blocking per
// subscriber: a full channel causes drop-oldest with a log warning so one
// slow consumer cannot stall the hub.
func (h *Hub) Publish(ev api.Event) {
	h.mu.Lock()
	subs := make([]chan api.Event, 0, len(h.subs))
	ids := make([]int, 0, len(h.subs))
	for id, ch := range h.subs {
		subs = append(subs, ch)
		ids = append(ids, id)
	}
	h.mu.Unlock()
	for i, ch := range subs {
		select {
		case ch <- ev:
		default:
			// Drop oldest to make room for the new event.
			select {
			case <-ch:
				slog.Warn("daemon: SSE subscriber slow, dropped oldest", "id", ids[i], "type", ev.Type)
			default:
			}
			select {
			case ch <- ev:
			default:
				slog.Warn("daemon: SSE publish failed after drop", "id", ids[i], "type", ev.Type)
			}
		}
	}
}

// Close shuts the hub: marks it closed and closes all subscriber channels.
// Further Publish calls are no-ops; Subscribe returns a closed channel.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	for id, ch := range h.subs {
		close(ch)
		delete(h.subs, id)
	}
}
