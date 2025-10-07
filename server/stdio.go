package server

import (
	"bufio"
	"context"
	"encoding/json"
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
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Result  any    `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type StdioServer struct {
	server    *DefaultServer
	sigChan   chan os.Signal
	errLogger *log.Logger
}

func ServeStdio(server *DefaultServer) error {
	s := &StdioServer{
		server:    server,
		sigChan:   make(chan os.Signal, 1),
		errLogger: log.New(os.Stderr, "", log.LstdFlags),
	}

	return s.serve()
}

func (s *StdioServer) serve() error {
	signal.Notify(s.sigChan, syscall.SIGTERM, syscall.SIGINT)

	reader := bufio.NewReader(os.Stdin)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		<-s.sigChan
		cancel()
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			if err := s.handleNextMessage(ctx, reader); err != nil {
				if err == io.EOF {
					return nil
				}
				s.errLogger.Printf("Error handling message: %v", err)
			}
		}
	}
}

func (s *StdioServer) handleNextMessage(
	ctx context.Context,
	reader *bufio.Reader,
) error {
	line, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	var request JSONRPCRequest
	if err := json.Unmarshal([]byte(line), &request); err != nil {
		s.writeError(nil, -32700, "Parse error")
		return fmt.Errorf("failed to parse JSON-RPC request: %w", err)
	}

	if request.JSONRPC != "2.0" {
		s.writeError(request.ID, -32600, "Invalid Request")
		return fmt.Errorf("invalid JSON-RPC version")
	}

	result, err := s.server.Request(ctx, request.Method, request.Params)
	if err != nil {
		s.writeError(request.ID, -32603, err.Error())
		return fmt.Errorf("request handling error: %w", err)
	}

	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      request.ID,
		Result:  result,
	}

	if err := s.writeResponse(response); err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}

	return nil
}

func (s *StdioServer) writeError(id any, code int, message string) {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}{
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

	if _, err := fmt.Fprintf(os.Stdout, "%s\n", responseBytes); err != nil {
		return err
	}

	return err
}
