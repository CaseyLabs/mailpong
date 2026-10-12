// Adapt structured log entries to the Contacts SDK-provided logger.
package backend

import (
	"encoding/json"
	coreapi "github.com/caseylabs/mailven/internal/core/api/v1"
	"github.com/rs/zerolog"
)

func contactsLogger(log coreapi.Logger) zerolog.Logger {
	if log == nil {
		return zerolog.Nop()
	}
	return zerolog.New(contactLogWriter{log: log})
}

type contactLogWriter struct{ log coreapi.Logger }

func (w contactLogWriter) Write(p []byte) (int, error) {
	var entry struct {
		Level   string `json:"level"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(p, &entry)
	msg := entry.Message
	var fields map[string]any
	if json.Unmarshal(p, &fields) == nil {
		delete(fields, "level")
		delete(fields, "message")
		delete(fields, "time")
		if len(fields) > 0 {
			if extra, err := json.Marshal(fields); err == nil {
				msg += " " + string(extra)
			}
		}
	}
	switch entry.Level {
	case "debug", "trace":
		w.log.Debug(msg)
	case "warn":
		w.log.Warn(msg)
	case "error", "fatal", "panic":
		w.log.Error(msg)
	default:
		w.log.Info(msg)
	}
	return len(p), nil
}
