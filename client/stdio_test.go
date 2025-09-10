package client

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockServer is a simple echo server for testing
const mockServerScript = `#!/bin/bash
    while IFS= read -r line; do
        echo "$line"
        echo "log message" >&2
    done
    `

func setupMockServer(t *testing.T) string {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "stdio-test-*")
	require.NoError(t, err)

	// Create mock server script
	scriptPath := filepath.Join(tmpDir, "mock-server.sh")
	err = os.WriteFile(scriptPath, []byte(mockServerScript), 0755)
	require.NoError(t, err)

	return scriptPath
}

func TestStdioTransport_Connect(t *testing.T) {
	scriptPath := setupMockServer(t)
	defer os.RemoveAll(filepath.Dir(scriptPath))

	transport := NewStdioTransport(scriptPath, nil)
	err := transport.Connect(context.Background())
	assert.NoError(t, err)

	err = transport.Disconnect()

	assert.NoError(t, err)

	assert.False(t, transport.IsConnected())
}

func TestStdioTransport_SendReceive(t *testing.T) {
	scriptPath := setupMockServer(t)
	defer os.RemoveAll(filepath.Dir(scriptPath))

	transport := NewStdioTransport(scriptPath, nil)
	err := transport.Connect(context.Background())
	require.NoError(t, err)
	defer transport.Disconnect()

	testMsg := &JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "test.method",
		Params:  map[string]any{"key": "value"},
		ID:      1,
	}

	err = transport.Send(context.Background(), testMsg)
	assert.NoError(t, err)

	msg, err := transport.Receive(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, testMsg.JSONRPC, msg.JSONRPC)
	assert.Equal(t, testMsg.Method, msg.Method)
	assert.Equal(t, testMsg.ID, msg.ID)
}
