//go:build windows

package ipc

import (
	"context"
	"fmt"
	"os/user"
	"strings"

	"github.com/Microsoft/go-winio"
)

// PipeServer implements the Server interface using Windows named pipes.
type PipeServer struct {
	*BaseServer
	pipeName string
}

// NewPipeServer creates a new Windows named pipe server.
func NewPipeServer(tokenMgr *TokenManager) *PipeServer {
	s := &PipeServer{
		BaseServer: NewBaseServer(tokenMgr),
	}
	// Pre-compute pipe name so Address() works before Start()
	if name, err := s.createPipeName(); err == nil {
		s.pipeName = name
		s.BaseServer.address = name
	}
	return s
}

// Start begins listening on the named pipe.
func (s *PipeServer) Start(ctx context.Context) error {
	pipeName, err := s.createPipeName()
	if err != nil {
		return fmt.Errorf("failed to create pipe name: %w", err)
	}
	s.pipeName = pipeName

	// Configure pipe security - only allow current user
	config := &winio.PipeConfig{
		SecurityDescriptor: "", // Default security allows only current user
		MessageMode:        false,
		InputBufferSize:    ReadBufferSize,
		OutputBufferSize:   ReadBufferSize,
	}

	listener, err := winio.ListenPipe(pipeName, config)
	if err != nil {
		return fmt.Errorf("failed to listen on pipe: %w", err)
	}

	s.SetListener(listener, pipeName)

	return s.AcceptLoop(ctx)
}

// Stop gracefully shuts down the server.
func (s *PipeServer) Stop() error {
	return s.BaseServer.Stop()
	// Named pipes are automatically cleaned up by Windows when closed
}

// createPipeName generates the named pipe path.
func (s *PipeServer) createPipeName() (string, error) {
	currentUser, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}

	// Use \\.\pipe\mailpong-{username} format
	// Sanitize backslashes from domain-joined usernames (e.g., DOMAIN\user)
	sanitized := strings.ReplaceAll(currentUser.Username, `\`, "_")
	pipeName := fmt.Sprintf(`\\.\pipe\mailpong-%s`, sanitized)

	return pipeName, nil
}

// NewServer creates a new platform-appropriate IPC server.
// On Windows, this returns a PipeServer.
func NewServer(tokenMgr *TokenManager) Server {
	return NewPipeServer(tokenMgr)
}
