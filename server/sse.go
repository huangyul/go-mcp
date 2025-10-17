package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"

	"github.com/google/uuid"
)

type SSEServer struct {
	mcpServer MCPServer
	baseURL   string
	sessions  sync.Map
	srv       *http.Server
}

type sseSession struct {
	writer  http.ResponseWriter
	flusher http.Flusher
	done    chan struct{}
}

func NewSSEServer(server MCPServer, baseURL string) *SSEServer {
	return &SSEServer{
		mcpServer: server,
		baseURL:   baseURL,
	}
}

// NewTestServer creates a test server for testing purposes
// It returns the SSEServer and a test server that can be closed when done
func NewTestServer(mcpServer MCPServer) (*SSEServer, *httptest.Server) {
	// Create SSE server with test server's URL as base
	sseServer := &SSEServer{
		mcpServer: mcpServer,
	}

	// Create test HTTP server
	testServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/sse":
				sseServer.handleSSE(w, r)

			case "/message":
				sseServer.handleMessage(w, r)
			default:
				http.NotFound(w, r)
			}
		}),
	)

	// Set base URL from test server
	sseServer.baseURL = testServer.URL

	return sseServer, testServer
}

func (s *SSEServer) Shutdown(ctx context.Context) error {
	if s.srv != nil {

		s.sessions.Range(func(key, value any) bool {
			if session, ok := value.(*sseSession); ok {
				close(session.done)
			}
			s.sessions.Delete(key)
			return true
		})

		return s.srv.Shutdown(ctx)
	}

	return nil
}

func (s *SSEServer) Start(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/sse", s.handleSSE)
	mux.HandleFunc("/message", s.handleMessage)

	s.srv = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return s.srv.ListenAndServe()
}

func (s *SSEServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
	}

	session := &sseSession{
		writer:  w,
		flusher: flusher,
		done:    make(chan struct{}),
	}
	sessionID := uuid.New().String()

	s.sessions.Store(sessionID, session)
	defer s.sessions.Delete(sessionID)

	// send endpoint event
	endpointEvent := fmt.Sprintf("event: endpoint\ndata: %s/message?sessionId=%s\n\n", s.baseURL, sessionID)

	fmt.Fprint(w, endpointEvent)
	flusher.Flush()

	<-r.Context().Done()
	close(session.done)
}

func (s *SSEServer) handleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSONRPCError(w, nil, -32600, "Method not allowed")
		return
	}

	sessionId := r.URL.Query().Get("sessionId")
	if sessionId == "" {
		s.writeJSONRPCError(w, nil, -32602, "Missing sessionId")
		return
	}

	sessionI, ok := s.sessions.Load(sessionId)
	if !ok {
		s.writeJSONRPCError(w, nil, -32602, "Invalid session ID")
		return
	}
	session := sessionI.(*sseSession)

	var request JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		s.writeJSONRPCError(w, nil, -32700, "Parse error")
		return
	}

	response := s.mcpServer.Request(r.Context(), request)

	data, _ := json.Marshal(response)
	fmt.Fprintf(session.writer, "event: message\ndata: %s\n\n", data)
	session.flusher.Flush()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(response)

}

func (s *SSEServer) writeJSONRPCError(
	w http.ResponseWriter,
	id any,
	code int,
	message string,
) {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
		},
	}

	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(response)
}

func (s *SSEServer) SendEventToSession(
	sessionID string,
	event any,
) error {
	sessionI, ok := s.sessions.Load(sessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	session := sessionI.(*sseSession)

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to parse event: %w", err)
	}

	select {
	case <-session.done:
		return fmt.Errorf("session closed")
	default:
		fmt.Fprintf(session.writer, "event: message\ndata: %s", data)
		session.flusher.Flush()
		return nil
	}
}
