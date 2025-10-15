package main

import (
	"log"
	"os"

	example "github.com/huangyul/go-mcp/examples/server"
	"github.com/huangyul/go-mcp/server"
)

func main() {
	mcpServer := server.NewDefaultServer("calculator", "1.0")

	mcpServer.HandleCallTool(example.HandleToolCall)
	mcpServer.HandleListTools(example.HandleListTools)

	if err := server.ServeStdio(mcpServer); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

}
