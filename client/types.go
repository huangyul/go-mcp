package client

import (
	"encoding/json"
	"fmt"

	"github.com/huangyul/go-mcp/shared"
)

type RequestID any

type JSONRPCMessage struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      RequestID     `json:"id,omitempty"`
	Method  string        `json:"method,omitempty"`
	Params  any           `json:"params,omitempty"`
	Result  any           `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type ClientConfig struct {
	Implementation shared.Implementation
	Capabilities   shared.ClientCapabilities
}

type TransportConfig struct{}

func (m *JSONRPCMessage) UnmarshalJSON(data []byte) error {
	type Alias JSONRPCMessage
	aux := &struct {
		ID json.RawMessage `json:"id,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(m),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if aux.ID != nil {
		var i int
		if err := json.Unmarshal(aux.ID, &i); err == nil {
			m.ID = i
			return nil
		}

		var s string
		if err := json.Unmarshal(aux.ID, &s); err == nil {
			m.ID = s
			return nil
		}

		return fmt.Errorf("id must be an integer or string")
	}

	return nil
}

func (m JSONRPCMessage) MarshalJSON() ([]byte, error) {
	type Alias JSONRPCMessage
	return json.Marshal((Alias)(m))
}
