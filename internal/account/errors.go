package account

import "errors"

var (
	// Validation errors
	ErrNameRequired        = errors.New("account name is required")
	ErrDisplayNameRequired = errors.New("display name is required")
	ErrEmailRequired       = errors.New("email address is required")
	ErrIMAPHostRequired    = errors.New("IMAP host is required")
	ErrSMTPHostRequired    = errors.New("SMTP host is required")
	ErrUsernameRequired    = errors.New("username is required")

	// Storage errors
	ErrAccountNotFound = errors.New("account not found")
	ErrAccountExists   = errors.New("account with this email already exists")

	// Identity errors
	ErrIdentityNotFound            = errors.New("identity not found")
	ErrCannotDeleteDefaultIdentity = errors.New("cannot delete the default identity")
	ErrInvalidReplyTo              = errors.New("reply-to must be a plain email address")

	// Connection errors
	ErrConnectionFailed = errors.New("failed to connect to server")
	ErrAuthFailed       = errors.New("authentication failed")
)
