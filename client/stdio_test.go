package client

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const mockServerScript = `#!/bin/bash
		while IFS= read -r line; do
				echo "$line"
				echo "log message" >&2
		done
		`

func setupMockServer(t *testing.T) string {
	tmpDir, err := os.MkdirTemp("", "stdio-test-*")
	require.NoError(t, err)

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
	assert.True(t, transport.IsConnected())

	err = transport.Disconnect()
	assert.NoError(t, err)
}

func TestStdioTransport_Receive(t *testing.T) {
	scriptPath := setupMockServer(t)
	defer os.RemoveAll(filepath.Dir(scriptPath))

	transport := NewStdioTransport(scriptPath, nil)
	err := transport.Connect(context.Background())
	require.NoError(t, err)
	defer transport.Disconnect()

	msg := &JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "test.method",
		ID:      1,
		Params: map[string]any{
			"key": "value",
		},
	}

	err = transport.Send(context.Background(), msg)
	require.NoError(t, err)

	res, err := transport.Receive(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, msg.ID, res.ID)
	assert.Equal(t, msg.Method, res.Method)
	assert.Equal(t, msg.JSONRPC, res.JSONRPC)
}

func TestStdioTransport_ContextCancellatiion(t *testing.T) {
	scriptPath := setupMockServer(t)
	defer os.RemoveAll(filepath.Dir(scriptPath))

	transport := NewStdioTransport(scriptPath, nil)
	err := transport.Connect(context.Background())
	require.NoError(t, err)
	defer transport.Disconnect()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = transport.Receive(ctx)
	assert.ErrorIs(t, err, context.Canceled)
}
