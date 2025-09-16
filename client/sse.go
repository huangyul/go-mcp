package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

type SSETransport struct {
	baseURL     string
	postURL     string
	httpClient  *http.Client
	eventSource *EventSource
	mu          sync.Mutex
	connected   bool
}

type EventSource struct {
	url        string
	httpClient *http.Client
	resp       *http.Response
	scanner    *EventScanner
	events     chan SSEEvent
	errors     chan error
	done       chan struct{}
	closeOnce  sync.Once
}

type SSEEvent struct {
	Type string
	Data string
}

type SSEOption func(*SSETransport)

func WithHTTPClient(client *http.Client) SSEOption {
	return func(s *SSETransport) {
		s.httpClient = client
	}
}

func NewSSETransport(baseUrl string, opts ...SSEOption) *SSETransport {
	t := &SSETransport{
		baseURL:    baseUrl,
		httpClient: http.DefaultClient,
	}

	for _, opt := range opts {
		opt(t)
	}

	return t
}

func (t *SSETransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.connected {
		return nil
	}

	es, err := newEventSource(ctx, t.baseURL, t.httpClient)
	if err != nil {
		return fmt.Errorf("failed to connect to SSE endpoint: %w", err)
	}

	select {
	case evt := <-es.events:
		if evt.Type != "endpoint" {
			es.Close()
			return fmt.Errorf("expected endpoint event, got: %s", evt.Type)
		}
		t.postURL = evt.Data
	case err := <-es.errors:
		es.Close()
		return fmt.Errorf("error waiting for endpoint: %w", err)
	case <-ctx.Done():
		es.Close()
		return ctx.Err()
	}

	t.eventSource = es
	t.connected = true
	return nil
}

func (t *SSETransport) Disconnect() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return nil
	}

	if t.eventSource != nil {
		t.eventSource.Close()
		t.eventSource = nil
	}

	t.connected = false
	return nil
}

func (t *SSETransport) Send(ctx context.Context, msg *JSONRPCMessage) error {
	if msg == nil {
		return errors.New("message cannot be nil")
	}

	t.mu.Lock()
	if !t.connected {
		return errors.New("not connected")
	}
	postURL := t.postURL
	t.mu.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		postURL,
		bytes.NewReader(data),
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf(
			"unexpected status code %d: %s",
			resp.StatusCode,
			string(body),
		)
	}

	return nil
}

func (t *SSETransport) Receive(ctx context.Context) (*JSONRPCMessage, error) {
	t.mu.Lock()
	if !t.connected || t.eventSource == nil {
		t.mu.Unlock()
		return nil, errors.New("not connected")
	}
	es := t.eventSource
	t.mu.Unlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case evt := <-es.events:
		if evt.Type != "message" {
			return nil, fmt.Errorf("unexpected event type: %s", evt.Type)
		}
		var msg JSONRPCMessage
		err := json.Unmarshal([]byte(evt.Data), &msg)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal message: %w", err)
		}
		return &msg, nil
	case err := <-es.errors:
		return nil, fmt.Errorf("SSE error: %w", err)
	}
}

func newEventSource(ctx context.Context, baseURL string, client *http.Client) (*EventSource, error) {
	es := &EventSource{
		url:        baseURL,
		httpClient: client,
		events:     make(chan SSEEvent, 10),
		errors:     make(chan error, 1),
		done:       make(chan struct{}),
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, es.url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	es.resp = resp
	es.scanner = NewEventScanner(resp.Body)
	go es.readEvents()
	return es, nil
}

func (es *EventSource) readEvents() {
	defer es.resp.Body.Close()

	for es.scanner.Scan() {
		select {
		case <-es.done:
			return
		case es.events <- es.scanner.Event():
		}
	}

	if err := es.scanner.Err(); err != nil {
		select {
		case <-es.done:
		case es.errors <- err:
		}
	}
}

func (es *EventSource) Close() error {
	es.closeOnce.Do(func() {
		close(es.done)
		if es.resp != nil && es.resp.Body != nil {
			es.resp.Body.Close()
		}
	})
	return nil
}

type EventScanner struct {
	scanner *bufio.Scanner
	current SSEEvent
	err     error
}

func NewEventScanner(r io.Reader) *EventScanner {
	return &EventScanner{
		scanner: bufio.NewScanner(r),
	}
}

func (s *EventScanner) Scan() bool {
	s.current = SSEEvent{}
	inEvent := false

	for s.scanner.Scan() {
		line := s.scanner.Text()

		if line == "" {
			if inEvent {
				return true
			}
			continue
		}

		inEvent = true

		if strings.HasPrefix(line, "event:") {
			s.current.Type = strings.TrimSpace(line[6:])
		} else if strings.HasPrefix(line, "data:") {
			if s.current.Data != "" {
				s.current.Data += "\n"
			}
			s.current.Data += strings.TrimSpace(line[5:])
		} else if strings.HasPrefix(line, ":") {
			continue
		} else {
			if strings.Contains(line, ":") {
				parts := strings.SplitN(line, ":", 2)
				field := strings.TrimSpace(parts[0])
				value := ""
				if len(parts) > 1 {
					value = strings.TrimSpace(parts[1])
				}

				switch field {
				case "event":
					s.current.Type = value
				case "data":
					if s.current.Data != "" {
						s.current.Data += "\n"
					}
					s.current.Data += value
				}
			}
		}
	}

	if inEvent {
		return true
	}

	s.err = s.scanner.Err()
	return false
}

func (s *EventScanner) Event() SSEEvent {
	return s.current
}

func (s *EventScanner) Err() error {
	return s.err
}
