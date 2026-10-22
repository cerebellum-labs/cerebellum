package events

import "testing"

// drained reports whether ch is closed once its buffered messages are read.
func drained(ch <-chan []byte) bool {
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return true
			}
		default:
			return false
		}
	}
}

func TestHubDelivers(t *testing.T) {
	t.Parallel()
	var h Hub
	a, stopA := h.Subscribe()
	defer stopA()
	b, stopB := h.Subscribe()
	defer stopB()

	h.Publish([]byte("one"))

	for name, ch := range map[string]<-chan []byte{"a": a, "b": b} {
		if got := string(<-ch); got != "one" {
			t.Errorf("subscriber %s got %q, want %q", name, got, "one")
		}
	}
}

func TestHubDropsSlowSubscriber(t *testing.T) {
	t.Parallel()
	var h Hub
	slow, stopSlow := h.Subscribe()
	defer stopSlow()
	fast, stopFast := h.Subscribe()
	defer stopFast()

	for range buffer + 1 {
		h.Publish([]byte("x"))
		<-fast
	}

	if !drained(slow) {
		t.Error("the subscriber that never read is still subscribed")
	}
	h.Publish([]byte("after"))
	if got := string(<-fast); got != "after" {
		t.Errorf("the reading subscriber got %q, want %q", got, "after")
	}
}

func TestHubListening(t *testing.T) {
	t.Parallel()
	var h Hub
	if h.Listening() {
		t.Error("listening with no subscribers")
	}
	_, stop := h.Subscribe()
	if !h.Listening() {
		t.Error("not listening with a subscriber")
	}
	stop()
	if h.Listening() {
		t.Error("listening after the last unsubscribe")
	}
}

func TestHubUnsubscribe(t *testing.T) {
	t.Parallel()
	var h Hub
	ch, stop := h.Subscribe()
	stop()
	stop() // A second call must not panic.

	if !drained(ch) {
		t.Error("channel still open after unsubscribing")
	}
	h.Publish([]byte("x")) // Must not panic on the closed channel.
}

func TestHubClose(t *testing.T) {
	t.Parallel()
	var h Hub
	ch, stop := h.Subscribe()
	h.Close()
	stop() // After Close it must not close the channel again.

	if !drained(ch) {
		t.Error("channel still open after Close")
	}
	late, _ := h.Subscribe()
	if !drained(late) {
		t.Error("a subscription after Close is open")
	}
}
