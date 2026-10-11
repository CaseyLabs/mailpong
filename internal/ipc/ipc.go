// Package ipc provides inter-process communication infrastructure
// for multi-window support in the MailPong email client.
//
// It supports Unix sockets on Linux/macOS and named pipes on Windows,
// enabling bidirectional communication between the main window and
// detached composer windows.
package ipc

import (
	"context"
	"encoding/json"
)

// Message represents an IPC message exchanged between main window and composers.
type Message struct {
	// ID is a unique identifier for request/response matching
	ID string `json:"id"`

	// Type is the message type constant (see message.go)
	Type string `json:"type"`

	// Token is the authentication token (included in first message from client)
	Token string `json:"token,omitempty"`

	// Payload contains type-specific data
	Payload json.RawMessage `json:"payload,omitempty"`

	// ReplyTo is set when this message is a response to another message
	ReplyTo string `json:"reply_to,omitempty"`

	// Error is set when the message represents an error response
	Error string `json:"error,omitempty"`
}

// MessageHandler is called when a message is received.
// The clientID identifies which client sent the message.
type MessageHandler func(clientID string, msg Message)

// Server represents an IPC server that accepts connections from composer windows.
type Server interface {
	// Start begins listening for connections.
	// It blocks until the context is cancelled or an error occurs.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the server, closing all client connections.
	Stop() error

	// Address returns the address clients should connect to.
	// For Unix sockets, this is the socket path.
	// For Windows named pipes, this is the pipe name.
	Address() string

	// OnMessage registers a handler for incoming messages.
	// Only one handler can be registered; subsequent calls replace the previous handler.
	OnMessage(handler MessageHandler)

	// Send sends a message to a specific client.
	Send(clientID string, msg Message) error

	// Broadcast sends a message to all connected clients.
	Broadcast(msg Message) error

	// Clients returns the IDs of all currently connected clients.
	Clients() []string
}

// Client represents an IPC client that connects to the main window's server.
type Client interface {
	// Connect establishes a connection to the server.
	// The token is sent as part of the authentication handshake.
	Connect(ctx context.Context, token string) error

	// Close gracefully closes the connection.
	Close() error

	// OnMessage registers a handler for incoming messages from the server.
	OnMessage(handler func(msg Message))

	// Send sends a message to the server.
	Send(msg Message) error

	// SendAndWait sends a message and waits for a response with matching ReplyTo.
	// Returns an error if the context is cancelled before a response is received.
	SendAndWait(ctx context.Context, msg Message) (*Message, error)
}

// ClientInfo contains metadata about a connected client.
type ClientInfo struct {
	// ID is the unique identifier for this client connection
	ID string

	// Authenticated indicates whether the client has successfully authenticated
	Authenticated bool
}
