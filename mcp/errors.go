package mcp

import "fmt"

// UnsupportedProtocolVersionError is returned when the server responds with a protocol version that the client doesn't supported.
type UnsupportedProtocolVersionError struct {
	Version string
}

func (e UnsupportedProtocolVersionError) Error() string {
	return fmt.Sprintf("Unsupported protocol version: %s", e.Version)
}

// Is impelements the errors.Is interface for better error handling
func (e UnsupportedProtocolVersionError) Is(target error) bool {
	_, ok := target.(UnsupportedProtocolVersionError)
	return ok
}

// IsUnsupportedProtocolVersionError check if an error is an UnsupportedProtocolVersionError
func IsUnsupportedProtocolVersionError(err error) bool {
	_, ok := err.(UnsupportedProtocolVersionError)
	return ok
}
