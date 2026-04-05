package mcpclient

import "fmt"

// NewStdioFactory returns a ClientFactory that creates stdio-based MCP clients.
func NewStdioFactory() ClientFactory {
	return func(config ServerConfig) (Client, error) {
		return NewStdioClient(config)
	}
}

// DefaultFactory returns a ClientFactory supporting all built-in transports.
// Currently supports: stdio.
// HTTP/SSE transport can be added when needed.
func DefaultFactory() ClientFactory {
	return func(config ServerConfig) (Client, error) {
		switch config.Transport {
		case "stdio", "":
			return NewStdioClient(config)
		default:
			return nil, fmt.Errorf("unsupported MCP transport: %q", config.Transport)
		}
	}
}
