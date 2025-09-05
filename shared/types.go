package shared

// Role represents the sender or recipient of messages and data in a conversation
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// LoggingLevel represents the serverity of a log message
type LoggingLevel string

const (
	LogLevelEmergency LoggingLevel = "emergency"
	LogLevelAlert     LoggingLevel = "alert"
	LogLevelCritical  LoggingLevel = "critical"
	LogLevelError     LoggingLevel = "error"
	LogLevelWarning   LoggingLevel = "warning"
	LogLevelNotice    LoggingLevel = "notice"
	LogLevelInfo      LoggingLevel = "info"
	LogLevelDebug     LoggingLevel = "debug"
)

// Implementation represents the name and version of a MCP implementation
type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ClientCapabilities represents capabilities a client may support
type ClientCapabilities struct {
	Experimental map[string]map[string]interface{} `json:"experimental,omitempty"`
	Sampling     map[string]interface{}            `json:"sampling,omitempty"`
	Roots        *RootCapabilities                 `json:"roots,omitempty"`
}

// RootCapabilities represents root-specific capabilities
type RootCapabilities struct {
	ListChanged bool `json:"listChanged"`
}

// ServerCapabilities represents capabilities the server may support
type ServerCapabilities struct {
	Experimental map[string]map[string]any `json:"experimental,omitempty"`
	Logging      map[string]any            `json:"logging,omitempty"`
	Prompts      *PromptCapabilities       `json:"prompts,omitempty"`
	Resources    *ResourceCapabilities     `json:"resources,omitempty"`
	Tools        *ToolCapabilities         `json:"tools,omitempty"`
}

type PromptCapabilities struct {
	ListChanged bool `json:"listChanged"`
}

type ResourceCapabilities struct {
	ListChanged bool `json:"listChanged"`
	Subscribe   bool `json:"subscribe"`
}

type ToolCapabilities struct {
	ListChanged bool `json:"listChanged"`
}
