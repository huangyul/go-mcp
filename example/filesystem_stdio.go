package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/huangyul/go-mcp/client"
)

type ContentType string

const (
	ContentTypeText  ContentType = "text"
	ContentTypeImage ContentType = "image"
)

type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type CallToolResult struct {
	Content []json.RawMessage `json:"content"`
	IsError bool              `json:"isError,omitempty"`
}

type FilesystemClient struct {
	transport *client.StdioTransport
}

func NewFilesystemClient() (*FilesystemClient, error) {
	transport := client.NewStdioTransport(
		"/home/huang/.nvm/versions/node/v22.19.0/bin/npx",
		[]string{
			"-y",
			"@modelcontextprotocol/server-filesystem",
			"/tmp",
		},
		client.WithStdioDir("/tmp"),
	)

	ctx := context.Background()
	if err := transport.Connect(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	return &FilesystemClient{transport: transport}, nil
}

func (fc *FilesystemClient) ListDirectory(ctx context.Context, path string) ([]string, error) {
	result, err := fc.callTool(ctx, "list_directory", map[string]any{
		"path": path,
	})
	if err != nil {
		return nil, err
	}

	if len(result.Content) == 0 {
		return nil, fmt.Errorf("no content returned")
	}

	var textContent TextContent
	if err := json.Unmarshal(result.Content[0], &textContent); err != nil {
		return nil, fmt.Errorf("failed to parse content: %w", err)
	}

	entries := strings.Split(strings.TrimSpace(textContent.Text), "\n")
	return entries, nil
}

func (fc *FilesystemClient) CreateDirectory(ctx context.Context, path string) error {
	_, err := fc.callTool(ctx, "create_directory", map[string]any{
		"path": path,
	})

	return err
}

func (fc *FilesystemClient) WriteFile(ctx context.Context, path, content string) error {
	_, err := fc.callTool(ctx, "write_file", map[string]any{
		"path":    path,
		"content": content,
	})

	return err
}

func (fc *FilesystemClient) callTool(ctx context.Context, name string, args map[string]any) (*CallToolResult, error) {
	msg := &client.JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "tools/call",
		Params: map[string]any{
			"name":      name,
			"arguments": args,
		},
		ID: 1,
	}

	err := fc.transport.Send(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	response, err := fc.transport.Receive(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to receive response: %w", err)
	}

	resultBytes, err := json.Marshal(response.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	var result CallToolResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to parse result: %w", err)
	}

	if result.IsError {
		return nil, fmt.Errorf("tool execution failed")
	}

	return &result, nil
}

func main() {
	client, err := NewFilesystemClient()
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()

	fmt.Println("listring /tmp diretory...")
	entries, err := client.ListDirectory(ctx, "/tmp")
	if err != nil {
		log.Fatalf("failed to list directory: %v", err)
	}

	for _, entry := range entries {
		fmt.Println(entry)
	}

	fmt.Println("\nCreating /tmp/mcp direcotry...")
	err = client.CreateDirectory(ctx, "/tmp/mcp")
	if err != nil {
		log.Fatalf("failed to create directory: %v", err)
	}

	fmt.Println("\nCreating and writing to /tmp/mcp/test.txt...")
	err = client.WriteFile(ctx, "/tmp/mcp/test.txt", "hello, go mcp")
	if err != nil {
		log.Fatalf("failed to write file: %v", err)
	}

	entries, err = client.ListDirectory(ctx, "/tmp/mcp")
	if err != nil {
		log.Fatalf("Failed to list directory: %v", err)
	}
	for _, entry := range entries {
		fmt.Println(entry)
	}
}
