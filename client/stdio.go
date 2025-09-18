package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
)

type StdioMCPClient struct {
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	stdout      *bufio.Reader
	requestID   atomic.Int64
	response    map[int64]chan *json.RawMessage
	mu          sync.Mutex
	done        chan struct{}
	initialized bool
}

func NewStdioMCPClient(
	command string,
	args ...string,
) (*StdioMCPClient, error) {
	cmd := exec.Command(command, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	client := &StdioMCPClient{
		cmd:      cmd,
		stdin:    stdin,
		stdout:   bufio.NewReader(stdout),
		response: make(map[int64]chan *json.RawMessage),
		done:     make(chan struct{}),
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	go client.readResponses()

	return client, nil
}

func (s *StdioMCPClient) Close() error {
	close(s.done)
	if err := s.stdin.Close(); err != nil {
		return fmt.Errorf("failed to close stdin: %w", err)
	}

	return s.cmd.Wait()
}

func (s *StdioMCPClient) readResponses() {
	for {
		select {
		case <-s.done:
			return
		default:
			line, err := s.stdout.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					fmt.Printf("error reading response: %v\n", err)
				}
				return
			}

			var response struct {
				ID     int64           `json:"id"`
				Result json.RawMessage `json:"result,omitempty"`
				Error  *struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				} `json:"error,omitempty"`
			}

			if err := json.Unmarshal([]byte(line), &response); err != nil {
				continue
			}

			s.mu.Lock()
			ch, ok := s.response[response.ID]
			s.mu.Unlock()

			if ok {
				if response.Error != nil {
					ch <- nil
				} else {
					ch <- &response.Result
				}
				s.mu.Lock()
				delete(s.response, response.ID)
				s.mu.Unlock()
			}
		}
	}
}

func (s *StdioMCPClient) sendRequest(
	ctx context.Context,
	method string,
	params any,
) (*json.RawMessage, error) {
	if !s.initialized && method != "initialized" {
		return nil, fmt.Errorf("client not initialized")
	}

	id := s.requestID.Add(1)

	request := struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int64  `json:"id"`
		Method  string `json:"method"`
		Params  any    `json:"params,omitempty"`
	}{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	responseChan := make(chan *json.RawMessage, 1)
	s.mu.Lock()
	s.response[id] = responseChan
	s.mu.Unlock()

	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	requestBytes = append(requestBytes, '\n')

	if _, err := s.stdin.Write(requestBytes); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	select {
	case <-ctx.Done():
		s.mu.Lock()
		delete(s.response, id)
		s.mu.Unlock()
		return nil, ctx.Err()
	case response := <-responseChan:
		if response == nil {
			return nil, fmt.Errorf("request failed")
		}
		return response, nil
	}
}

// Initialize implements MCPClient.
func (s *StdioMCPClient) Initialize(ctx context.Context, capabilities ClientCapabilities, clientInfo Implementation, protocolVersion string) (*InitializeResult, error) {
	params := struct {
		Capabilities    ClientCapabilities `json:"capabilities"`
		ClientInfo      Implementation     `json:"clientInfo"`
		ProtocolVersion string             `json:"protocolVersion"`
	}{
		Capabilities:    capabilities,
		ClientInfo:      clientInfo,
		ProtocolVersion: protocolVersion,
	}

	response, err := s.sendRequest(ctx, "initialize", params)
	if err != nil {
		return nil, err
	}

	var result InitializeResult
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	s.initialized = true
	return &result, nil
}

// ListResources implements MCPClient.
func (s *StdioMCPClient) ListResources(ctx context.Context, cursor *string) (*ListResourcesResult, error) {
	params := struct {
		Cursor *string `json:"cursor,omitempty"`
	}{
		Cursor: cursor,
	}

	response, err := s.sendRequest(ctx, "resources/list", params)
	if err != nil {
		return nil, err
	}

	var result ListResourcesResult
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// ReadResource implements MCPClient.
func (s *StdioMCPClient) ReadResource(ctx context.Context, uri string) (*ReadResourceResult, error) {
	params := struct {
		URI string `uri:"uri"`
	}{
		URI: uri,
	}

	response, err := s.sendRequest(ctx, "resources/read", params)
	if err != nil {
		return nil, err
	}

	var result ReadResourceResult
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// Subscribe implements MCPClient.
func (s *StdioMCPClient) Subscribe(ctx context.Context, uri string) error {
	params := struct {
		URI string `json:"uri"`
	}{
		URI: uri,
	}

	_, err := s.sendRequest(ctx, "resources/subscribe", params)
	return err
}

func (c *StdioMCPClient) Unsubscribe(ctx context.Context, uri string) error {
	params := struct {
		URI string `json:"uri"`
	}{
		URI: uri,
	}

	_, err := c.sendRequest(ctx, "resources/unsubscribe", params)
	return err
}

func (c *StdioMCPClient) ListPrompts(
	ctx context.Context,
	cursor *string,
) (*ListPromptsResult, error) {
	params := struct {
		Cursor *string `json:"cursor,omitempty"`
	}{
		Cursor: cursor,
	}

	response, err := c.sendRequest(ctx, "prompts/list", params)
	if err != nil {
		return nil, err
	}

	var result ListPromptsResult
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

func (c *StdioMCPClient) GetPrompt(
	ctx context.Context,
	name string,
	arguments map[string]string,
) (*GetPromptResult, error) {
	params := struct {
		Name      string            `json:"name"`
		Arguments map[string]string `json:"arguments,omitempty"`
	}{
		Name:      name,
		Arguments: arguments,
	}

	response, err := c.sendRequest(ctx, "prompts/get", params)
	if err != nil {
		return nil, err
	}

	var result GetPromptResult
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

func (c *StdioMCPClient) ListTools(
	ctx context.Context,
	cursor *string,
) (*ListToolsResult, error) {
	params := struct {
		Cursor *string `json:"cursor,omitempty"`
	}{
		Cursor: cursor,
	}

	response, err := c.sendRequest(ctx, "tools/list", params)
	if err != nil {
		return nil, err
	}

	var result ListToolsResult
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

func (c *StdioMCPClient) CallTool(
	ctx context.Context,
	name string,
	arguments map[string]interface{},
) (*CallToolResult, error) {
	params := struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments,omitempty"`
	}{
		Name:      name,
		Arguments: arguments,
	}

	response, err := c.sendRequest(ctx, "tools/call", params)
	if err != nil {
		return nil, err
	}

	var result CallToolResult
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

func (c *StdioMCPClient) SetLevel(
	ctx context.Context,
	level LoggingLevel,
) error {
	params := struct {
		Level LoggingLevel `json:"level"`
	}{
		Level: level,
	}

	_, err := c.sendRequest(ctx, "logging/setLevel", params)
	return err
}

func (c *StdioMCPClient) Complete(
	ctx context.Context,
	ref interface{},
	argument CompleteArgument,
) (*CompleteResult, error) {
	params := struct {
		Ref      interface{}      `json:"ref"`
		Argument CompleteArgument `json:"argument"`
	}{
		Ref:      ref,
		Argument: argument,
	}

	response, err := c.sendRequest(ctx, "completion/complete", params)
	if err != nil {
		return nil, err
	}

	var result CompleteResult
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}
