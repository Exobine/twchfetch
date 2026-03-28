// Package logging provides a session-scoped logger backed by charmbracelet/log.
// The log file is truncated on each new session start.
package logging

import (
	"fmt"
	"os"

	"github.com/charmbracelet/log"
)

// Logger is the global logger. It is nil until Init is called.
var Logger *log.Logger

// Init initialises the logger. logPath is the path of the log file (e.g.
// "twchfetch.log"). The file is truncated on every call, wiping the previous
// session's output.
func Init(logPath string) error {
	f, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("logging: create log file: %w", err)
	}

	Logger = log.New(f)
	Logger.SetLevel(log.DebugLevel)
	Logger.SetReportTimestamp(true)
	Logger.SetTimeFormat("15:04:05")

	Logger.Info("Session started", "pid", os.Getpid())
	return nil
}

// Info logs at Info level (no-op if Init was not called).
func Info(msg interface{}, keyvals ...interface{}) {
	if Logger != nil {
		Logger.Info(msg, keyvals...)
	}
}

// Warn logs at Warn level.
func Warn(msg interface{}, keyvals ...interface{}) {
	if Logger != nil {
		Logger.Warn(msg, keyvals...)
	}
}

// Error logs at Error level.
func Error(msg interface{}, keyvals ...interface{}) {
	if Logger != nil {
		Logger.Error(msg, keyvals...)
	}
}

// Debug logs at Debug level.
func Debug(msg interface{}, keyvals ...interface{}) {
	if Logger != nil {
		Logger.Debug(msg, keyvals...)
	}
}
