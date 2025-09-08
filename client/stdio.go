package client

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

// StdioConfig holds configuration options for stdio stransport
type StdioConfig struct {
	// Command is the server process to execute
	Command string
	// Args are the arguments to pass to the command
	Args []string
	// Dir is the working directory for the command
	Dir string
	// Env is the environment variables for the command
	Env []string
	// StderrHandler is called for each line written to stderr
	StderrHandler func(string)
}

// StdioOption represents an option for configuring the stdio transport
type StdioOption func(*StdioConfig)

// WithStdioDir set the working directory for the server process
func WithStdioDir(dir string) StdioOption {
	return func(c *StdioConfig) {
		c.Dir = dir
	}
}

// WithStdioEnv sets environment variables for the server process
func WithStdioEnv(env []string) StdioOption {
	return func(c *StdioConfig) {
		c.Env = env
	}
}

// WithStdioStderrHandler sets a handler for stderr output
func WithStdioStderrHandler(handler func(string)) StdioOption {
	return func(c *StdioConfig) {
		c.StderrHandler = handler
	}
}

// Update the StdioTransport struct to include the stderr handler
type StdioTransport struct {
	cmd           *exec.Cmd
	stdin         io.WriteCloser
	stdout        *bufio.Reader
	stderr        io.ReadCloser
	stderrDone    chan struct{}
	processExit   chan error
	mu            sync.Mutex
	connected     bool
	stderrHandler func(string)
}

// Update NewStdioTransport to store the stderr handler
func NewStdioTransport(command string, args []string, opts ...StdioOption) *StdioTransport {

	config := &StdioConfig{
		Command: command,
		Args:    args,
		// Default stderr handler just writes to os.Stderr
		StderrHandler: func(line string) {
			fmt.Fprintln(os.Stderr, line)
		},
	}

	for _, opt := range opts {
		opt(config)
	}

	return &StdioTransport{
		cmd: &exec.Cmd{
			Path: config.Command,
			Args: append([]string{config.Command}, config.Args...),
			Dir:  config.Dir,
			Env:  config.Env,
		},
		stderrHandler: config.StderrHandler,
		stderrDone:    make(chan struct{}),
		processExit:   make(chan error, 1),
	}
}

// Update handleStderr to use the configured handler
func (t *StdioTransport) handleStderr() {
	defer close(t.stderrDone)

	scanner := bufio.NewScanner(t.stderr)
	for scanner.Scan() {
		line := scanner.Text()
		if t.stderrHandler != nil {
			t.stderrHandler(line)
		}
	}
}

// Connect starts the server process and establishes stdio communication
func (t *StdioTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.connected {
		return nil
	}

	stdin, err := t.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %s", err.Error())
	}
	t.stdin = stdin

	stdout, err := t.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %s", err.Error())
	}
	t.stdout = bufio.NewReader(stdout)

	stderr, err := t.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %s", err.Error())
	}
	t.stderr = stderr

	// start the process
	if err := t.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	// Handle process exit
	go func() {
		t.processExit <- t.cmd.Wait()
	}()

	t.connected = true
	return nil
}

// Disconnet stop the server process and close all pipes
func (t *StdioTransport) Disconnect() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return nil
	}

	// Close stdin to signal EOF to the process
	if t.stdin != nil {
		t.stdin.Close()
	}

	// Wait for process to exit with timeout
	select {
	case err := <-t.processExit:
		if err != nil && !errors.Is(err, os.ErrProcessDone) {
			return fmt.Errorf("process exited with error: %w", err)
		}
	case <-time.After(time.Second * 5):
		if err := t.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
	}

	// Wait for stderr handler to finish
	<-t.stderrDone

	t.connected = false
	return nil

}

// Send sends a JSON-RPC message to the server process's stdin
func (t *StdioTransport) Send(ctx context.Context, msg *JSONRPCMessage) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return errors.New("not connected")
	}

	// Check if process has exited
	select {
	case err := <-t.processExit:
		return fmt.Errorf("process has exited: %w", err)
	default:
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	data = append(data, '\n')

	done := make(chan error, 1)
	go func() {
		_, err := t.stdin.Write(data)
		done <- err
	}()

	// Wait for write to complete or context to be canceled
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return fmt.Errorf("failed to write message: %w", err)
		}
	}

	return nil
}

// Receive receives a JSON-RPC message from the server process's stdout
func (t *StdioTransport) Receive(ctx context.Context) (*JSONRPCMessage, error) {
	t.mu.Lock()
	if !t.connected {
		t.mu.Unlock()
		return nil, errors.New("not connected")
	}
	t.mu.Unlock()

	select {
	case err := <-t.processExit:
		return nil, fmt.Errorf("process has exited: %w", err)
	default:
	}

	type readResult struct {
		msg *JSONRPCMessage
		err error
	}

	done := make(chan readResult, 1)

	go func() {
		str, err := t.stdout.ReadString('\n')
		if err != nil {
			done <- readResult{
				err: fmt.Errorf("failed to read message: %w", err),
			}
			return
		}
		var msg JSONRPCMessage
		if err := json.Unmarshal([]byte(str), &msg); err != nil {
			done <- readResult{
				err: fmt.Errorf("failed to Unmarshal message: %w", err),
			}
			return
		}
		done <- readResult{msg: &msg}

	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-done:
		if res.err != nil {
			return nil, fmt.Errorf("failed to read message: %w", res.err)
		}
		return res.msg, nil
	}
}

// IsConnected returns whether the transport is currently connected
func (t *StdioTransport) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.connected
}
