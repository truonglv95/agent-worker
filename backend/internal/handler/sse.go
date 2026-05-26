package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"ai-agent-backend/internal/sse"
)

// SSEHandler handles the Server-Sent Events stream endpoint.
type SSEHandler struct {
	sseBus *sse.SSEBus
}

// NewSSEHandler constructs an SSEHandler.
func NewSSEHandler(sseBus *sse.SSEBus) *SSEHandler {
	return &SSEHandler{sseBus: sseBus}
}

// Stream handles GET /sse/workflows/{id}.
// It subscribes to the SSEBus for the given workflow and streams events until
// the client disconnects or the context is cancelled.
func (h *SSEHandler) Stream(w http.ResponseWriter, r *http.Request) {
	rawID := chi.URLParam(r, "id")
	workflowID, err := strconv.Atoi(rawID)
	if err != nil || workflowID < 1 {
		http.Error(w, `{"error":"invalid workflow id"}`, http.StatusBadRequest)
		return
	}

	// Verify the response writer supports flushing.
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	// SSE required headers.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	// Send an initial ping so the client knows the connection is live.
	_, _ = w.Write([]byte("data: {\"type\":\"connected\"}\n\n"))
	flusher.Flush()

	ch := h.sseBus.Subscribe(workflowID)
	defer h.sseBus.Unsubscribe(workflowID, ch)

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			log.Printf("[sse] client disconnected from workflow %d", workflowID)
			return
		case event, open := <-ch:
			if !open {
				return
			}
			b, err := json.Marshal(event)
			if err != nil {
				log.Printf("[sse] marshal error: %v", err)
				continue
			}
			_, _ = w.Write([]byte("data: "))
			_, _ = w.Write(b)
			_, _ = w.Write([]byte("\n\n"))
			flusher.Flush()
		}
	}
}
