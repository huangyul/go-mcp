package main

import (
	"fmt"
	"log"
	"os"

	example "github.com/huangyul/go-mcp/examples/server"
	"github.com/huangyul/go-mcp/server"
)

func main() {
	mcpServer := server.NewDefaultServer("calculator", "1.0")

	mcpServer.HandleCallTool(example.HandleToolCall)
	mcpServer.HandleListTools(example.HandleListTools)

	fmt.Fprintf(os.Stdout, "server is running\name: %s\nversion: %s\n\n", "calculator", "1.0")

	if err := server.ServeStdio(mcpServer); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}
