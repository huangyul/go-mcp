package client

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

// Client represent an MCP client
type Client struct {
	config     ClientConfig
	transport  Transport
	nextReqID  atomic.Int64
	handlers   map[string]NotificationHandler
	handlersmu sync.RWMutex
}

// New create a new MCP client
func New(config ClientConfig, transport Transport) *Client {
	return &Client{
		config:    config,
		transport: transport,
		handlers:  make(map[string]NotificationHandler),
	}
}

// nextRequestID generates the next request ID
func (c *Client) nextRequestID() RequestID {
	return c.nextReqID.Add(1)
}

// Connect establishes a connection with the server
func (c *Client) Connect(ctx context.Context) error {
	if err := c.transport.Connect(ctx); err != nil {
		return err
	}

	// Send initialize request
	initReq := &JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      c.nextRequestID(),
		Method:  "initialize",
		Params: map[string]any{
			"clientInfo":      c.config.Implementation,
			"capabilities":    c.config.Capabilities,
			"protocolVersion": "1.0", // update with actual version
		},
	}

	if err := c.transport.Send(ctx, initReq); err != nil {
		return err
	}

	// wait for initialize response
	resp, err := c.transport.Receive(ctx)
	if err != nil {
		return err
	}

	if resp.Error != nil {
		return errors.New(resp.Error.Message)
	}

	notifications := &JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}

	return c.transport.Send(ctx, notifications)
}

// RegisterNotificationHandler register a handler for a specific notification method
func (c *Client) RegisterNotificationHandler(method string, handler NotificationHandler) {
	c.handlersmu.Lock()
	defer c.handlersmu.Unlock()
	c.handlers[method] = handler
}

func (c *Client) HandleNotification(ctx context.Context, msg *JSONRPCMessage) error {
	c.handlersmu.RLock()
	handler, ok := c.handlers[msg.Method]
	c.handlersmu.RUnlock()

	if !ok {
		return nil
	}

	return handler(ctx, msg)
}

// Transport defines interface for different transport implementations
type Transport interface {
	Connect(context.Context) error
	Disconnect() error
	Send(context.Context, *JSONRPCMessage) error
	Receive(context.Context) (*JSONRPCMessage, error)
}

// NotificationHandler represents a handler for incoming notifications
type NotificationHandler func(context.Context, *JSONRPCMessage) error
