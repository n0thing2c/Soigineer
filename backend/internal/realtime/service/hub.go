package service

import (
	"context"
	"encoding/json"

	sharedDomain "github.com/n0thing2c/Soigineer/internal/shared/domain"
)

const (
	hubLogQueueSize   = 1024
	hubAlertQueueSize = 256
)

type Hub struct {
	authorizer Authorizer

	register   chan *Client
	unregister chan *Client
	done       chan struct{}

	logQueue   chan sharedDomain.ProcessedLogEvent
	alertQueue chan sharedDomain.AlertEvent

	logClients   map[*Client]struct{}
	alertClients map[*Client]struct{}
}

func NewHub(authorizer Authorizer) *Hub {
	if authorizer == nil {
		authorizer = AllowAllAuthorizer{}
	}

	return &Hub{
		authorizer: authorizer,

		register:   make(chan *Client),
		unregister: make(chan *Client),
		done:       make(chan struct{}),

		logQueue:   make(chan sharedDomain.ProcessedLogEvent, hubLogQueueSize),
		alertQueue: make(chan sharedDomain.AlertEvent, hubAlertQueueSize),

		logClients:   make(map[*Client]struct{}),
		alertClients: make(map[*Client]struct{}),
	}
}

func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case client := <-h.register:
			if client.stream == StreamLogs {
				h.logClients[client] = struct{}{}
			} else if client.stream == StreamAlerts {
				h.alertClients[client] = struct{}{}
			}

		case client := <-h.unregister:
			h.removeClient(client)

		case event := <-h.logQueue:
			h.broadcastLog(event)

		case event := <-h.alertQueue:
			h.broadcastAlert(event)

		case <-ctx.Done():
			close(h.done)
			h.closeAllClients()
			return
		}
	}
}

func (h *Hub) Register(client *Client) {
	select {
	case h.register <- client:
	case <-h.done:
		client.Close()
	}
}

func (h *Hub) Unregister(client *Client) {
	select {
	case h.unregister <- client:
	case <-h.done:
	}
}

func (h *Hub) removeClient(client *Client) {
	if _, ok := h.logClients[client]; ok {
		delete(h.logClients, client)
		close(client.send)
		return
	}

	if _, ok := h.alertClients[client]; ok {
		delete(h.alertClients, client)
		close(client.send)
		return
	}
}

func (h *Hub) closeAllClients() {
	for client := range h.logClients {
		delete(h.logClients, client)
		close(client.send)
	}

	for client := range h.alertClients {
		delete(h.alertClients, client)
		close(client.send)
	}
}

func (h *Hub) PublishLog(ctx context.Context, event sharedDomain.ProcessedLogEvent) error {
	select {
	case h.logQueue <- event:
		return nil
	case <-h.done:
		return context.Canceled
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *Hub) PublishAlert(ctx context.Context, event sharedDomain.AlertEvent) error {
	select {
	case h.alertQueue <- event:
		return nil
	case <-h.done:
		return context.Canceled
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *Hub) broadcastLog(event sharedDomain.ProcessedLogEvent) {
	jsonEvent, err := json.Marshal(event)
	if err != nil {
		return
	}

	for client := range h.logClients {
		if !client.subscription.MatchLog(event) {
			continue
		}

		if !h.authorizer.AuthorizeLog(client.principal, event) {
			continue
		}

		select {
		case client.send <- jsonEvent:
		default:
			h.removeClient(client)
		}
	}
}

func (h *Hub) broadcastAlert(event sharedDomain.AlertEvent) {
	jsonEvent, err := json.Marshal(event)
	if err != nil {
		return
	}

	for client := range h.alertClients {
		if !client.subscription.MatchAlert(event) {
			continue
		}

		if !h.authorizer.AuthorizeAlert(client.principal, event) {
			continue
		}

		select {
		case client.send <- jsonEvent:
		default:
			h.removeClient(client)
		}
	}
}
