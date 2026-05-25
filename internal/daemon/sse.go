package daemon

import (
	"log/slog"
	"sync"

	"github.com/venkatkrishna07/mkdev/internal/api"
)

const hubChannelCap = 256

type Hub struct {
	mu     sync.Mutex
	nextID int
	subs   map[int]chan api.Event
	closed bool
}

func NewHub() *Hub {
	return &Hub{subs: map[int]chan api.Event{}}
}

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
