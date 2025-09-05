// Package mcp defines the core types and interfaces for the Model Content Protocol(MCP)
package mcp

import (
	"encoding/json"

	"github.com/yosida95/uritemplate/v3"
)

type MCPMethod string

const (
	// MethodInitialize initiates connection and negotiates protocol capabilities.
	// https://modelcontextprotocol.io/specification/2024-11-05/basic/lifecycle/#initialization
	MethodInitialize MCPMethod = "initialize"

	// MethodPing verifies connection liveness between client and server.
	// https://modelcontextprotocol.io/specification/2024-11-05/basic/utilities/ping/
	MethodPing MCPMethod = "ping"

	// MethodResourcesList lists all available server resources.
	// https://modelcontextprotocol.io/specification/2024-11-05/server/resources/
	MethodResourcesList MCPMethod = "resources/list"

	// MethodResourcesTemplatesList provides URI templates for constructing resource URIs.
	// https://modelcontextprotocol.io/specification/2024-11-05/server/resources/
	MethodResourcesTemplatesList MCPMethod = "resources/templates/list"

	// MethodResourcesRead retrieves content of a specific resource by URI.
	// https://modelcontextprotocol.io/specification/2024-11-05/server/resources/
	MethodResourcesRead MCPMethod = "resources/read"

	// MethodPromptsList lists all available prompt templates.
	// https://modelcontextprotocol.io/specification/2024-11-05/server/prompts/
	MethodPromptsList MCPMethod = "prompts/list"

	// MethodPromptsGet retrieves a specific prompt template with filled parameters.
	// https://modelcontextprotocol.io/specification/2024-11-05/server/prompts/
	MethodPromptsGet MCPMethod = "prompts/get"

	// MethodToolsList lists all available executable tools.
	// https://modelcontextprotocol.io/specification/2024-11-05/server/tools/
	MethodToolsList MCPMethod = "tools/list"

	// MethodToolsCall invokes a specific tool with provided parameters.
	// https://modelcontextprotocol.io/specification/2024-11-05/server/tools/
	MethodToolsCall MCPMethod = "tools/call"

	// MethodSetLogLevel configures the minimum log level for client
	// https://modelcontextprotocol.io/specification/2025-03-26/server/utilities/logging
	MethodSetLogLevel MCPMethod = "logging/setLevel"

	// MethodNotificationResourcesListChanged notifies when the list of available resources changes.
	// https://modelcontextprotocol.io/specification/2025-03-26/server/resources#list-changed-notification
	MethodNotificationResourcesListChanged = "notifications/resources/list_changed"

	MethodNotificationResourceUpdated = "notifications/resources/updated"

	// MethodNotificationPromptsListChanged notifies when the list of available prompt templates changes.
	// https://modelcontextprotocol.io/specification/2025-03-26/server/prompts#list-changed-notification
	MethodNotificationPromptsListChanged = "notifications/prompts/list_changed"

	// MethodNotificationToolsListChanged notifies when the list of available tools changes.
	// https://spec.modelcontextprotocol.io/specification/2024-11-05/server/tools/list_changed/
	MethodNotificationToolsListChanged = "notifications/tools/list_changed"
)

type URITemplate struct {
	*uritemplate.Template
}

func (u *URITemplate) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.Raw())
}

func (u *URITemplate) UnmarshalJSON(data []byte) error {
	var res string
	err := json.Unmarshal(data, &res)
	if err != nil {
		return err
	}

	template, err := uritemplate.New(res)
	if err != nil {
		return err
	}

	u.Template = template
	return nil
}
