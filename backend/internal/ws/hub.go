package ws

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Hub fans alert payloads out to every connected browser.
type Hub struct {
	mu      sync.Mutex
	clients map[*client]struct{}
}

type client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
	once sync.Once
}

// New returns an empty hub.
func New() *Hub {
	return &Hub{clients: make(map[*client]struct{})}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// The console is a separate origin during local development.
	CheckOrigin: func(*http.Request) bool { return true },
}

// ServeHTTP upgrades the request and keeps the socket until it closes.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &client{
		hub:  h,
		conn: conn,
		send: make(chan []byte, 32),
	}
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()

	go c.writePump()
	c.readPump()
}

// Broadcast queues msg for every client without waiting on the network.
// A slow client drops the message instead of stalling location updates.
func (h *Hub) Broadcast(msg []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		select {
		case c.send <- msg:
		default:
		}
	}
}

func (c *client) readPump() {
	defer c.close()
	c.conn.SetReadLimit(4096)
	_ = c.conn.SetReadDeadline(time.Now().Add(70 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(70 * time.Second))
	})
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (c *client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *client) close() {
	c.once.Do(func() {
		c.hub.mu.Lock()
		if _, ok := c.hub.clients[c]; ok {
			delete(c.hub.clients, c)
			close(c.send)
		}
		c.hub.mu.Unlock()
		_ = c.conn.Close()
	})
}
