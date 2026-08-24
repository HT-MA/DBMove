package sse

import (
	"encoding/json"
	"sync"
)

// Event is a single server-sent event.
type Event struct {
	Type string `json:"-"`
	Data any    `json:"data"`
}

// Hub fans out events to per-task subscribers.
type Hub struct {
	mu   sync.RWMutex
	subs map[uint64]map[chan Event]struct{}
}

func NewHub() *Hub {
	return &Hub{subs: map[uint64]map[chan Event]struct{}{}}
}

// Subscribe registers a channel for a task and returns an unsubscribe func.
func (h *Hub) Subscribe(taskID uint64) (chan Event, func()) {
	ch := make(chan Event, 256)
	h.mu.Lock()
	if h.subs[taskID] == nil {
		h.subs[taskID] = map[chan Event]struct{}{}
	}
	h.subs[taskID][ch] = struct{}{}
	h.mu.Unlock()

	var once sync.Once
	unsub := func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			if m := h.subs[taskID]; m != nil {
				delete(m, ch)
				if len(m) == 0 {
					delete(h.subs, taskID)
				}
			}
			close(ch)
		})
	}
	return ch, unsub
}

// Publish sends an event to all subscribers of a task without blocking.
func (h *Hub) Publish(taskID uint64, ev Event) {
	h.mu.RLock()
	subs := h.subs[taskID]
	chans := make([]chan Event, 0, len(subs))
	for ch := range subs {
		chans = append(chans, ch)
	}
	h.mu.RUnlock()
	for _, ch := range chans {
		select {
		case ch <- ev:
		default: // drop for slow consumers
		}
	}
}

// Encode renders an event in SSE wire format.
func Encode(ev Event) []byte {
	data, err := json.Marshal(ev.Data)
	if err != nil {
		data = []byte("{}")
	}
	eventType := ev.Type
	if eventType == "" {
		eventType = "message"
	}
	return []byte("event: " + eventType + "\ndata: " + string(data) + "\n\n")
}
