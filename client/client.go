package client

import (
	"context"
	"sync"
	"sync/atomic"
)

type Client struct {
	config     ClientConfig
	transport  Transport
	nextReqID  atomic.Int64
	handlers   map[string]NotificationHandler
	handlersmu sync.RWMutex
}

type Transport interface {
	Connect(context.Context) error
	Disconnect() error
	Send(context.Context, *JSONRPCMessage) error
	Receive(context.Context) (*JSONRPCMessage, error)
}

type NotificationHandler func(context.Context, *JSONRPCMessage) error

func New(config ClientConfig, transport Transport) *Client {
	return &Client{
		config:    config,
		transport: transport,
		handlers:  make(map[string]NotificationHandler),
	}
}

func (c *Client) Connect(ctx context.Context) error {
	if err := c.transport.Connect(ctx); err != nil {
		return err
	}

	initReq := &JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "initialize",
		ID:      c.nextRequestID(),
		Params: map[string]interface{}{
			"clientInfo":      c.config.Implementation,
			"capabilities":    c.config.Capabilities,
			"protocolVersion": "1.0", // Update with actual version
		},
	}

	if err := c.transport.Send(ctx, initReq); err != nil {
		return err
	}

	resp, err := c.transport.Receive(ctx)
	if err != nil {
		return err
	}
	if resp.Error != nil {
		return err
	}

	initNotif := &JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}

	return c.transport.Send(ctx, initNotif)
}

func (c *Client) nextRequestID() RequestID {
	return c.nextReqID.Add(1)
}

func (c *Client) RegisterNotificationHandler(method string, handler NotificationHandler) {
	c.handlersmu.Lock()
	defer c.handlersmu.Unlock()

	c.handlers[method] = handler
}

func (c *Client) handleNotification(ctx context.Context, msg *JSONRPCMessage) error {
	c.handlersmu.RLock()
	handler, ok := c.handlers[msg.Method]
	c.handlersmu.RUnlock()

	if !ok {
		return nil
	}

	return handler(ctx, msg)
}
