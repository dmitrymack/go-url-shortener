package audit

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fakeObserver records every event it receives, guarded by a mutex since
// Log delivers to it from its own goroutine.
type fakeObserver struct {
	id string

	mu     sync.Mutex
	events []Event
}

func (o *fakeObserver) GetID() string { return o.id }

func (o *fakeObserver) Update(event Event) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.events = append(o.events, event)
}

func (o *fakeObserver) received() []Event {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]Event(nil), o.events...)
}

func TestLog_NotifyDeliversToRegisteredObservers(t *testing.T) {
	l := NewLog(zap.NewNop())
	obs := &fakeObserver{id: "obs1"}
	l.Register(obs)

	event := NewEvent(ActionShorten, "user1", "https://example.com")
	l.Notify(event)

	require.Eventually(t, func() bool {
		return len(obs.received()) == 1
	}, time.Second, time.Millisecond)
	assert.Equal(t, event, obs.received()[0])
}

func TestLog_DeregisterStopsDelivery(t *testing.T) {
	l := NewLog(zap.NewNop())
	obs := &fakeObserver{id: "obs1"}
	l.Register(obs)
	l.Deregister(obs)

	l.Notify(NewEvent(ActionShorten, "user1", "https://example.com"))

	time.Sleep(50 * time.Millisecond)
	assert.Empty(t, obs.received(), "a deregistered observer must not receive events")
}

func TestLog_StopDrainsBufferedEventsBeforeReturning(t *testing.T) {
	l := NewLog(zap.NewNop())
	obs := &fakeObserver{id: "obs1"}
	l.Register(obs)

	const eventCount = 5
	for i := 0; i < eventCount; i++ {
		l.Notify(NewEvent(ActionShorten, "user1", "https://example.com"))
	}

	done := make(chan struct{})
	go func() {
		l.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Stop did not return in time")
	}

	assert.Len(t, obs.received(), eventCount, "Stop must wait for every already-buffered event to be delivered")
}

func TestLog_NotifyAfterStopIsNoop(t *testing.T) {
	l := NewLog(zap.NewNop())
	obs := &fakeObserver{id: "obs1"}
	l.Register(obs)
	l.Stop()

	assert.NotPanics(t, func() {
		l.Notify(NewEvent(ActionShorten, "user1", "https://example.com"))
	})
}
