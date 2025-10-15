package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
)

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      any           `json:"id"`
	Result  any           `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type StdioServer struct {
	server    MCPServer
	signChan  chan os.Signal
	errLogger *log.Logger
	done      chan struct{}
}

func ServeStdio(server MCPServer) error {
	s := &StdioServer{
		server:    server,
		signChan:  make(chan os.Signal, 1),
		errLogger: log.New(os.Stderr, "", log.LstdFlags),
		done:      make(chan struct{}),
	}

	signal.Notify(s.signChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-s.signChan
		close(s.done)
	}()

	return s.serve()
}

func (s *StdioServer) serve() error {

	reader := bufio.NewReader(os.Stdin)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-s.done
		cancel()
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:

			readChan := make(chan string, 1)
			errChan := make(chan error, 1)

			go func() {
				line, err := reader.ReadString('\n')
				if err != nil {
					errChan <- err
					return
				}
				readChan <- line
			}()

			select {
			case <-ctx.Done():
				return nil
			case err := <-errChan:
				if errors.Is(err, io.EOF) {
					return nil
				}
				s.errLogger.Printf("Error reading input: %v", err)
				return err
			case line := <-readChan:
				if err := s.handleMessage(ctx, line); err != nil {
					if err == io.EOF {
						return nil
					}
					s.errLogger.Printf("Error handling message: %v", err)
				}
			}
		}
	}
}

func (s *StdioServer) handleMessage(ctx context.Context, line string) error {
	var request JSONRPCRequest
	if err := json.Unmarshal([]byte(line), &request); err != nil {
		s.writeError(nil, -32700, "Parse error")
		return fmt.Errorf("failed to parse JSON-RPC request: %v", err)
	}

	if request.JSONRPC != "2.0" {
		s.writeError(nil, -32600, "Invalid version")
		return fmt.Errorf("invalid JSON-RPC version")
	}

	result, err := s.server.Request(ctx, request.Method, request.Params)
	if err != nil {
		s.writeError(nil, -32603, err.Error())
		return fmt.Errorf("request handling error: %w", err)
	}

	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      request.ID,
		Result:  result,
	}

	return s.writeResponse(response)
}

func (s *StdioServer) writeError(
	id any,
	code int,
	message string) {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
		},
	}

	s.writeResponse(response)
}

func (s *StdioServer) writeResponse(response JSONRPCResponse) error {
	responseBytes, err := json.Marshal(response)
	if err != nil {
		return err
	}

	responseBytes = append(responseBytes, '\n')
	_, err = os.Stdout.Write(responseBytes)
	return err
}
