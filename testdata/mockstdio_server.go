package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int64         `json:"id"`
	Result  any           `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func main() {
	fmt.Println("start read stdin")

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var request JSONRPCRequest
		err := json.Unmarshal(scanner.Bytes(), &request)
		if err != nil {
			continue
		}

		response := handleRequest(request)
		responseBytes, err := json.Marshal(response)
		if err == nil {
			responseBytes = append(responseBytes, '\n')
			os.Stdout.Write(responseBytes)
		}
	}

}

func handleRequest(request JSONRPCRequest) JSONRPCResponse {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      request.ID,
	}

	switch request.Method {
	case "initialize":
		response.Result = map[string]interface{}{
			"protocolVersion": "1.0",
			"serverInfo": map[string]interface{}{
				"name":    "mock-server",
				"version": "1.0.0",
			},
			"capabilities": map[string]interface{}{
				"prompts": map[string]interface{}{
					"listChanged": true,
				},
				"resources": map[string]interface{}{
					"listChanged": true,
					"subscribe":   true,
				},
				"tools": map[string]interface{}{
					"listChanged": true,
				},
			},
		}
	case "ping":
		response.Result = struct{}{}
	case "resources/list":
		response.Result = map[string]interface{}{
			"resources": []map[string]interface{}{
				{
					"name": "test-resource",
					"uri":  "test://resource",
				},
			},
		}
	case "resources/read":
		response.Result = map[string]interface{}{
			"contents": []map[string]interface{}{
				{
					"text": "test content",
					"uri":  "test://resource",
				},
			},
		}
	case "resources/subscribe", "resources/unsubscribe":
		response.Result = struct{}{}
	case "prompts/list":
		response.Result = map[string]interface{}{
			"prompts": []map[string]interface{}{
				{
					"name": "test-prompt",
				},
			},
		}
	case "prompts/get":
		response.Result = map[string]interface{}{
			"messages": []map[string]interface{}{
				{
					"role": "assistant",
					"content": map[string]interface{}{
						"type": "text",
						"text": "test message",
					},
				},
			},
		}
	case "tools/list":
		response.Result = map[string]interface{}{
			"tools": []map[string]interface{}{
				{
					"name": "test-tool",
					"inputSchema": map[string]interface{}{
						"type": "object",
					},
				},
			},
		}
	case "tools/call":
		response.Result = map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": "tool result",
				},
			},
		}
	case "logging/setLevel":
		response.Result = struct{}{}
	case "completion/complete":
		response.Result = map[string]interface{}{
			"completion": map[string]interface{}{
				"values": []string{"test completion"},
			},
		}
	default:
		response.Error = &JSONRPCError{
			Code:    -32601,
			Message: "Method not found",
		}
	}

	return response
}
