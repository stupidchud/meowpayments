// Package hub manages WebSocket connections and broadcasts messages to rooms.
//
// Rooms:
//   - "payment:<uuid>"  - receives events for a single payment (customer-facing)
//   - "global"          - receives all payment events (operator-facing)
package hub

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

// Conn wraps a websocket connection with its subscribed rooms.
type Conn struct {
	ws    *websocket.Conn
	rooms map[string]struct{}
	send  chan []byte
	mu    sync.Mutex
}

func newConn(ws *websocket.Conn, rooms ...string) *Conn {
	c := &Conn{
		ws:    ws,
		rooms: make(map[string]struct{}, len(rooms)),
		send:  make(chan []byte, 64),
	}
	for _, r := range rooms {
		c.rooms[r] = struct{}{}
	}
	return c
}

type subscribeMsg struct {
	conn  *Conn
	rooms []string
}

type broadcastMsg struct {
	room string
	data []byte
}

// Hub is the central WebSocket broker.
type Hub struct {
	mu          sync.RWMutex
	rooms       map[string]map[*Conn]struct{}
	subscribe   chan subscribeMsg
	unsubscribe chan *Conn
	broadcast   chan broadcastMsg
}

// New creates a new Hub. Call Run(ctx) to start it.
func New() *Hub {
	return &Hub{
		rooms:       make(map[string]map[*Conn]struct{}),
		subscribe:   make(chan subscribeMsg, 64),
		unsubscribe: make(chan *Conn, 64),
		broadcast:   make(chan broadcastMsg, 256),
	}
}

// Run processes subscription and broadcast events until ctx is cancelled.
func (h *Hub) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case msg := <-h.subscribe:
			h.mu.Lock()
			for _, room := range msg.rooms {
				if h.rooms[room] == nil {
					h.rooms[room] = make(map[*Conn]struct{})
				}
				h.rooms[room][msg.conn] = struct{}{}
			}
			h.mu.Unlock()

		case conn := <-h.unsubscribe:
			h.mu.Lock()
			for room, conns := range h.rooms {
				delete(conns, conn)
				if len(conns) == 0 {
					delete(h.rooms, room)
				}
			}
			h.mu.Unlock()
			close(conn.send)

		case msg := <-h.broadcast:
			h.mu.RLock()
			conns := h.rooms[msg.room]
			// Collect recipients while holding read lock
			targets := make([]*Conn, 0, len(conns))
			for c := range conns {
				targets = append(targets, c)
			}
			h.mu.RUnlock()
			for _, c := range targets {
				select {
				case c.send <- msg.data:
				default:
					// Slow consumer - drop message to avoid blocking the hub.
				}
			}

		case <-ticker.C:
			// Send ping to all connections
			pingData, _ := json.Marshal(Message{Type: TypePing, Timestamp: time.Now()})
			h.mu.RLock()
			var allConns []*Conn
			seen := make(map[*Conn]struct{})
			for _, conns := range h.rooms {
				for c := range conns {
					if _, ok := seen[c]; !ok {
						allConns = append(allConns, c)
						seen[c] = struct{}{}
					}
				}
			}
			h.mu.RUnlock()
			for _, c := range allConns {
				select {
				case c.send <- pingData:
				default:
				}
			}
		}
	}
}

// Broadcast sends a Message to all subscribers in the given room.
func (h *Hub) Broadcast(room string, msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	select {
	case h.broadcast <- broadcastMsg{room: room, data: data}:
	default:
	}
}

// Register adds a WebSocket connection and subscribes it to the given rooms.
// It starts the write pump and returns the Conn so the caller can start a read pump.
func (h *Hub) Register(ws *websocket.Conn, rooms ...string) *Conn {
	conn := newConn(ws, rooms...)
	h.subscribe <- subscribeMsg{conn: conn, rooms: rooms}
	go conn.writePump()
	return conn
}

// Unregister removes a connection from all rooms.
func (h *Hub) Unregister(conn *Conn) {
	h.unsubscribe <- conn
}

// writePump drains conn.send and writes to the WebSocket.
func (c *Conn) writePump() {
	for data := range c.send {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_ = c.ws.Write(ctx, websocket.MessageText, data)
		cancel()
	}
}
