package client

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockServer is a simple echo server for testing
const mockServerScript = `@echo off
setlocal enabledelayedexpansion
:loop
set /p line=
if errorlevel 1 goto :eof
echo !line!
echo log message 1>&2
goto loop
`

func setupMockServer(t *testing.T) string {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "stdio-test-*")
	assert.NoError(t, err)

	// Create mock server script
	scriptPath := filepath.Join(tmpDir, "mock-server.bat")
	err = os.WriteFile(scriptPath, []byte(mockServerScript), 0755)
	assert.NoError(t, err)

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
