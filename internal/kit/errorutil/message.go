// Package errorutil extracts public messages without exposing private error
// references used by an IPC transport. It preserves wrapping error context.
package errorutil

import "strings"

type userMessage interface {
	error
	UserMessage() string
}

func Message(err error) string {
	if err == nil {
		return ""
	}
	if public, ok := err.(userMessage); ok {
		return public.UserMessage()
	}
	message := err.Error()
	switch wrapped := err.(type) {
	case interface{ Unwrap() []error }:
		for _, child := range wrapped.Unwrap() {
			if child != nil {
				message = strings.ReplaceAll(message, child.Error(), Message(child))
			}
		}
	case interface{ Unwrap() error }:
		if child := wrapped.Unwrap(); child != nil {
			message = strings.ReplaceAll(message, child.Error(), Message(child))
		}
	}
	return message
}
