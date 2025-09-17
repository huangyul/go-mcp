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

type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type ListToolsResult struct {
	Tools []Tool `json:"tools"`
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
	fsClient, err := NewFilesystemClient()
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()

	fmt.Println("Available Tools:")
	fmt.Println("---------------")

	msg := &client.JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "tools/list",
		ID:      1,
	}

	err = fsClient.transport.Send(ctx, msg)
	if err != nil {
		log.Fatalf("failed to send tools/list response: %v", err)
	}

	response, err := fsClient.transport.Receive(ctx)
	if err != nil {
		log.Fatalf("failed to receive tools/list: %v", err)
	}

	resultBytes, err := json.Marshal(response.Result)
	if err != nil {
		log.Fatalf("failed to marshal result: %v", err)
	}

	var result ListToolsResult
	err = json.Unmarshal(resultBytes, &result)
	if err != nil {
		log.Fatalf("failed to parse tools list: %v", err)
	}

	for _, tool := range result.Tools {
		fmt.Printf("\n📦 %s\n", tool.Name)
		fmt.Printf("   %s\n", tool.Description)
		fmt.Printf("   Input: %s\n", tool.InputSchema)
	}

	fmt.Println("\nDemo Operations:")
	fmt.Println("---------------")

	fmt.Println("\n📂 listring /tmp diretory...")
	entries, err := fsClient.ListDirectory(ctx, "/tmp")
	if err != nil {
		log.Fatalf("failed to list directory: %v", err)
	}

	for _, entry := range entries {
		fmt.Println(entry)
	}

	fmt.Println("\n📂 Creating /tmp/mcp direcotry...")
	err = fsClient.CreateDirectory(ctx, "/tmp/mcp")
	if err != nil {
		log.Fatalf("failed to create directory: %v", err)
	}

	fmt.Println("📝 Creating and writing to /tmp/mcp/test.txt...")
	err = fsClient.WriteFile(ctx, "/tmp/mcp/test.txt", "hello, go mcp\n")
	if err != nil {
		log.Fatalf("failed to write file: %v", err)
	}

	fmt.Println("\n📂 Listing /tmp/mcp directory:")
	entries, err = fsClient.ListDirectory(ctx, "/tmp/mcp")
	if err != nil {
		log.Fatalf("Failed to list directory: %v", err)
	}
	for _, entry := range entries {
		fmt.Println(entry)
	}
}
