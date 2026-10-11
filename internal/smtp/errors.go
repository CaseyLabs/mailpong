// Package smtp provides SMTP client functionality for MailPong
package smtp

import "errors"

var (
	// ErrNotConnected indicates the client is not connected
	ErrNotConnected = errors.New("not connected")

	// ErrAuthFailed indicates authentication failed
	ErrAuthFailed = errors.New("authentication failed")

	// ErrNoRecipients indicates no recipients were specified
	ErrNoRecipients = errors.New("no recipients specified")

	// ErrInvalidAddress indicates an invalid email address
	ErrInvalidAddress = errors.New("invalid email address")

	// ErrMessageTooLarge indicates the message exceeds the server's size limit
	ErrMessageTooLarge = errors.New("message too large")

	// ErrRejected indicates the server rejected the message
	ErrRejected = errors.New("message rejected by server")

	// ErrTimeout indicates a timeout occurred
	ErrTimeout = errors.New("operation timed out")
)
