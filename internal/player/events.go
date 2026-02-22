package player

import (
	"encoding/json"
	"fmt"

	"github.com/gorilla/websocket"
)

// EventHandler connects to the go-librespot WebSocket event stream.
type EventHandler struct {
	conn *websocket.Conn
	Ch   chan Event
}

// NewEventHandler connects to ws://localhost:{port}/events.
func NewEventHandler(port int) (*EventHandler, error) {
	url := fmt.Sprintf("ws://localhost:%d/events", port)
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, fmt.Errorf("connecting to events WebSocket: %w", err)
	}
	return &EventHandler{
		conn: conn,
		Ch:   make(chan Event, 32),
	}, nil
}

// Start begins reading events in a background goroutine.
// Events are sent to h.Ch. The goroutine exits when the connection closes.
func (h *EventHandler) Start() {
	go func() {
		defer close(h.Ch)
		for {
			_, msg, err := h.conn.ReadMessage()
			if err != nil {
				return
			}
			var ev Event
			if err := json.Unmarshal(msg, &ev); err != nil {
				continue
			}
			h.Ch <- ev
		}
	}()
}

// Close closes the WebSocket connection.
func (h *EventHandler) Close() {
	h.conn.Close()
}
