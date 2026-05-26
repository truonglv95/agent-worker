package sse

import (
	"sync"

	"ai-agent-backend/internal/models"
)

// SSEBus is a lightweight pub/sub hub for Server-Sent Events per workflow.
// Each workflow gets its own set of subscriber channels so that multiple
// browser tabs can listen simultaneously.
type SSEBus struct {
	mu       sync.RWMutex
	channels map[int][]chan models.SSEEvent
}

// NewSSEBus allocates and returns a ready-to-use SSEBus.
func NewSSEBus() *SSEBus {
	return &SSEBus{
		channels: make(map[int][]chan models.SSEEvent),
	}
}

// Subscribe registers a new channel for the given workflowID and returns it.
// The caller must eventually call Unsubscribe to avoid leaking goroutines.
func (b *SSEBus) Subscribe(workflowID int) chan models.SSEEvent {
	ch := make(chan models.SSEEvent, 64)
	b.mu.Lock()
	b.channels[workflowID] = append(b.channels[workflowID], ch)
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes a previously subscribed channel and closes it.
func (b *SSEBus) Unsubscribe(workflowID int, ch chan models.SSEEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	subs, ok := b.channels[workflowID]
	if !ok {
		return
	}

	updated := make([]chan models.SSEEvent, 0, len(subs))
	for _, s := range subs {
		if s != ch {
			updated = append(updated, s)
		}
	}

	if len(updated) == 0 {
		delete(b.channels, workflowID)
	} else {
		b.channels[workflowID] = updated
	}
	close(ch)
}

// Publish broadcasts an SSEEvent to all subscribers of the given workflowID.
// It uses a non-blocking send so a slow consumer never stalls the orchestrator.
func (b *SSEBus) Publish(workflowID int, event models.SSEEvent) {
	b.mu.RLock()
	subs := b.channels[workflowID]
	b.mu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- event:
		default:
			// Drop the event rather than block; the buffer is generous (64).
		}
	}
}
