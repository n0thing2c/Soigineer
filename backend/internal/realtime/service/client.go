package service

import (
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 54 * time.Second
	maxMessageSize = 512
	sendBufferSize = 256
)

type Client struct {
	hub          *Hub
	conn         *websocket.Conn
	send         chan []byte
	stream       StreamType
	principal    Principal
	subscription Subscription
}

func NewClient(
	hub *Hub,
	conn *websocket.Conn,
	stream StreamType,
	principal Principal,
	sub Subscription,
) *Client {
	send := make(chan []byte, sendBufferSize)
	return &Client{
		hub:          hub,
		conn:         conn,
		send:         send,
		stream:       stream,
		principal:    principal,
		subscription: sub,
	}
}

func (c *Client) ReadLoop() {
	defer func() {
		c.hub.Unregister(c)
		c.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)

	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))

	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			break
		}
	}
}

func (c *Client) WriteLoop() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) Close() {
	_ = c.conn.Close()
}
