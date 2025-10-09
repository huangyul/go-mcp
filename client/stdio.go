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

type StdioConfig struct {
	Command       string
	Args          []string
	Dir           string
	Env           []string
	StderrHandler func(string)
}

type StdioOption func(*StdioConfig)

func WithStdioDir(dir string) StdioOption {
	return func(c *StdioConfig) {
		c.Dir = dir
	}
}

func WithStdioEnv(env []string) StdioOption {
	return func(c *StdioConfig) {
		c.Env = env
	}
}

func WithStdioStderrHandler(handler func(string)) StdioOption {
	return func(c *StdioConfig) {
		c.StderrHandler = handler
	}
}

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

func NewStdioTransport(
	command string,
	args []string,
	opts ...StdioOption,
) *StdioTransport {
	config := &StdioConfig{
		Command: command,
		Args:    args,
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

func (t *StdioTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.connected {
		return nil
	}

	stdin, err := t.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	t.stdin = stdin

	stdout, err := t.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	t.stdout = bufio.NewReader(stdout)

	stderr, err := t.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	t.stderr = stderr

	if err := t.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	go func() {
		t.processExit <- t.cmd.Wait()
	}()

	go t.handleStderr()

	t.connected = true
	return nil
}

func (t *StdioTransport) Disconnect() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return nil
	}

	if t.stdin != nil {
		t.stdin.Close()
	}

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

	<-t.stderrDone

	t.connected = false
	return nil
}

func (t *StdioTransport) Send(ctx context.Context, msg *JSONRPCMessage) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return fmt.Errorf("not connected")
	}

	select {
	case <-t.processExit:
		return fmt.Errorf("process has exited")
	default:
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to parse msg: %w", err)
	}

	data = append(data, '\n')

	done := make(chan error, 1)

	go func() {
		_, err := t.stdin.Write(data)
		done <- err
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return fmt.Errorf("failed to send msg: %w", err)
		}
	}

	return nil
}

func (t *StdioTransport) Receive(ctx context.Context) (*JSONRPCMessage, error) {
	t.mu.Lock()
	if !t.connected {
		t.mu.Unlock()
		return nil, errors.New("not connected")
	}
	t.mu.Unlock()

	// Check if process has exited
	select {
	case err := <-t.processExit:
		return nil, fmt.Errorf("process has exited: %w", err)
	default:
	}

	// Create channel for the read result
	type readResult struct {
		msg *JSONRPCMessage
		err error
	}
	done := make(chan readResult, 1)

	go func() {
		// Read a line from stdout
		line, err := t.stdout.ReadString('\n')
		if err != nil {
			done <- readResult{nil, fmt.Errorf("failed to read message: %w", err)}
			return
		}

		fmt.Println("---- Receive msg1: ----")
		fmt.Printf("---- %s ----\n", line)

		// Parse JSON message
		var msg JSONRPCMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			done <- readResult{nil, fmt.Errorf("failed to unmarshal message: %w", err)}
			return
		}

		fmt.Println("---- Receive msg2: ----")
		fmt.Printf("---- %v ----\n", msg)

		done <- readResult{&msg, nil}
	}()

	// Wait for read to complete or context to be canceled
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-done:
		return result.msg, result.err
	}
}

func (t *StdioTransport) IsConnected() bool {
	return t.connected
}
