package cluster

import (
	"strings"

	"github.com/rs/zerolog"
)

// ZerologWriter wraps a zerolog.Logger to implement io.Writer interface
// This allows us to use zerolog as the output for standard log.Logger
type ZerologWriter struct {
	logger zerolog.Logger
}

// Write implements io.Writer interface and logs to zerolog
func (z *ZerologWriter) Write(p []byte) (n int, err error) {
	msg := string(p)

	// Remove trailing newlines
	msg = strings.TrimSuffix(msg, "\n")

	// Try to parse the log level from memberlist's prefix
	switch {
	case strings.HasPrefix(msg, "[DEBUG] "):
		msg = strings.TrimPrefix(msg, "[DEBUG] ")
		z.logger.Debug().Msg(msg)
	case strings.HasPrefix(msg, "[INFO] "):
		msg = strings.TrimPrefix(msg, "[INFO] ")
		z.logger.Info().Msg(msg)
	case strings.HasPrefix(msg, "[WARN] "):
		msg = strings.TrimPrefix(msg, "[WARN] ")
		z.logger.Warn().Msg(msg)
	case strings.HasPrefix(msg, "[ERR] "):
		msg = strings.TrimPrefix(msg, "[ERR] ")
		z.logger.Error().Msg(msg)
	default:
		// No recognized prefix, log as error
		z.logger.Error().Msg(msg)
	}

	return len(p), nil
}
