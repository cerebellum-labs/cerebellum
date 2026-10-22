// Package events fans messages out to the clients listening in this
// process, like the dashboards connected over WebSocket.
package events

import "sync"

// buffer is how far a subscriber may fall behind before it is dropped.
// Events come in bursts of a few (a sync of ten decisions), and a dropped
// dashboard reconnects and reloads, so a small buffer is enough.
const buffer = 64

// Hub delivers every published message to every subscriber. The zero value
// is ready to use.
type Hub struct {
	mu     sync.Mutex
	subs   map[chan []byte]struct{}
	closed bool
}

// Subscribe returns a channel of the messages published from now on and a
// function that ends the subscription. The channel is closed when the
// subscriber falls too far behind, when the hub closes, or by the function.
func (h *Hub) Subscribe() (<-chan []byte, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	ch := make(chan []byte, buffer)
	if h.closed {
		close(ch)
		return ch, func() {}
	}
	if h.subs == nil {
		h.subs = make(map[chan []byte]struct{})
	}
	h.subs[ch] = struct{}{}
	return ch, func() { h.remove(ch) }
}

// Publish sends msg to every subscriber without waiting. A subscriber whose
// buffer is full is dropped rather than allowed to hold up the others.
// Subscribers must not modify msg.
func (h *Hub) Publish(msg []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for ch := range h.subs {
		select {
		case ch <- msg:
		default:
			delete(h.subs, ch)
			close(ch)
		}
	}
}

// Listening reports whether anyone is subscribed, so a publisher can skip
// building a message nobody would get.
func (h *Hub) Listening() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs) > 0
}

// Close ends every subscription and refuses new ones. It is meant for
// shutdown, when the connections behind the subscribers have to let go.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.closed = true
	for ch := range h.subs {
		delete(h.subs, ch)
		close(ch)
	}
}

func (h *Hub) remove(ch chan []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Already gone if it was dropped or the hub closed: closing twice panics.
	if _, ok := h.subs[ch]; ok {
		delete(h.subs, ch)
		close(ch)
	}
}
