package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/huangyul/go-mcp/client"
	"github.com/huangyul/go-mcp/mcp"
)

func main() {
	c, err := client.NewStdioMCPClient(
		"go",
		"run",
		"github.com/mark3labs/mcp-filesystem-server@latest",
		".",
		"/tmp",
	)

	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	initResult, err := c.Initialize(ctx, mcp.ClientCapabilities{}, mcp.Implementation{
		Name:    "example-client",
		Version: "1.0.0",
	}, "1.0")
	if err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}
	fmt.Printf(
		"Initialized with server: %s %s\n\n",
		initResult.ServerInfo.Name,
		initResult.ServerInfo.Version,
	)

	// List Tools
	fmt.Println("Listing available tools...")
	tools, err := c.ListTools(ctx, nil)
	if err != nil {
		log.Fatalf("Failed to list tools: %v", err)
	}
	for _, tool := range tools.Tools {
		fmt.Printf("- %s: %s\n", tool.Name, tool.Description)
	}
	fmt.Println()

	// List allowed directories
	fmt.Println("Listing allowed directories...")
	result, err := c.CallTool(ctx, "list_allowed_directories", nil)
	if err != nil {
		log.Fatalf("Failed to list allowed directories: %v", err)
	}
	printToolResult(result)
	fmt.Println()

	// List /tmp
	fmt.Println("Listing /tmp directory...")
	result, err = c.CallTool(ctx, "list_directory", map[string]interface{}{
		"path": "/tmp",
	})
	if err != nil {
		log.Fatalf("Failed to list directory: %v", err)
	}
	printToolResult(result)
	fmt.Println()

	// Create mcp directory
	fmt.Println("Creating /tmp/mcp directory...")
	result, err = c.CallTool(ctx, "create_directory", map[string]interface{}{
		"path": "/tmp/mcp",
	})
	if err != nil {
		log.Fatalf("Failed to create directory: %v", err)
	}
	printToolResult(result)
	fmt.Println()

	// Create hello.txt
	fmt.Println("Creating /tmp/mcp/hello.txt...")
	result, err = c.CallTool(ctx, "write_file", map[string]interface{}{
		"path":    "/tmp/mcp/hello.txt",
		"content": "Hello World",
	})
	if err != nil {
		log.Fatalf("Failed to create file: %v", err)
	}
	printToolResult(result)
	fmt.Println()

	// Verify file contents
	fmt.Println("Reading /tmp/mcp/hello.txt...")
	result, err = c.CallTool(ctx, "read_file", map[string]interface{}{
		"path": "/tmp/mcp/hello.txt",
	})
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}
	printToolResult(result)

	// Get file info
	fmt.Println("Getting info for /tmp/mcp/hello.txt...")
	result, err = c.CallTool(ctx, "get_file_info", map[string]interface{}{
		"path": "/tmp/mcp/hello.txt",
	})
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}
	printToolResult(result)

}

func printToolResult(result *mcp.CallToolResult) {
	for _, content := range result.Content {
		if textContext, ok := content.(mcp.TextContent); ok {
			fmt.Println(textContext.Text)
		} else {
			jsonBytes, _ := json.MarshalIndent(content, "", "  ")
			fmt.Println(string(jsonBytes))
		}
	}
}
