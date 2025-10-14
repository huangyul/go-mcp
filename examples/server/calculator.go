package server

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/huangyul/go-mcp/mcp"
	"github.com/huangyul/go-mcp/server"
)

type CalculationError struct {
	Message string
}

func (c *CalculationError) Error() string {
	return c.Message
}

type Calculator struct {
	server *server.DefaultServer
}

func NewCalculator() *Calculator {
	server := server.NewDefaultServer("calculator", "1.0")
	cal := &Calculator{server: server}

	server.HandleListTools(cal.handleListTools)
	server.HandleCallTool(cal.handleToolCall)

	return cal
}

func (c *Calculator) handleListTools(
	ctx context.Context,
	cursor *string,
) (*mcp.ListToolsResult, error) {
	return &mcp.ListToolsResult{
		Tools: []mcp.Tool{
			{
				Name:        "add",
				Description: "Add two numbers",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"a": map[string]interface{}{
							"type":        "number",
							"description": "First number",
						},
						"b": map[string]interface{}{
							"type":        "number",
							"description": "Second number",
						},
					},
				},
			},
			{
				Name:        "subtract",
				Description: "Subtract two numbers",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"a": map[string]interface{}{
							"type":        "number",
							"description": "First number",
						},
						"b": map[string]interface{}{
							"type":        "number",
							"description": "Second number",
						},
					},
				},
			},
			{
				Name:        "multiply",
				Description: "Multiply two numbers",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"a": map[string]interface{}{
							"type":        "number",
							"description": "First number",
						},
						"b": map[string]interface{}{
							"type":        "number",
							"description": "Second number",
						},
					},
				},
			},
			{
				Name:        "divide",
				Description: "Divide two numbers",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"a": map[string]interface{}{
							"type":        "number",
							"description": "First number (dividend)",
						},
						"b": map[string]interface{}{
							"type":        "number",
							"description": "Second number (divisor)",
						},
					},
				},
			},
		},
	}, nil
}

func (c *Calculator) handleToolCall(
	ctx context.Context,
	name string,
	args map[string]interface{},
) (*mcp.CallToolResult, error) {
	// Extract arguments
	a, ok := args["a"].(float64)
	if !ok {
		return nil, &CalculationError{Message: "parameter 'a' must be a number"}
	}
	b, ok := args["b"].(float64)
	if !ok {
		return nil, &CalculationError{Message: "parameter 'b' must be a number"}
	}

	var result float64

	switch name {
	case "add":
		result = a + b
	case "subtract":
		result = a - b
	case "multiply":
		result = a * b
	case "divide":
		if b == 0 {
			return nil, &CalculationError{Message: "division by zero"}
		}
		result = a / b
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}

	// Create response
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("%.2f", result),
			},
		},
	}, nil
}

func (c *Calculator) Serve() error {
	return server.ServeStdio(c.server)
}

func main() {
	cal := NewCalculator()

	if err := cal.Serve(); err != nil {
		log.Printf("Server error: %v\n", err)
		os.Exit(1)
	}
}
