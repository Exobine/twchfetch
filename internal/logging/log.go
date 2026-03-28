// Package logging provides a session-scoped logger backed by charmbracelet/log.
//
// On Init the log file is truncated (wiping the previous session's output) and
// this process's PID is written to a companion .session file. A custom writer
// checks the session file before every write so that, when two instances of the
// app overlap, only the most-recently started instance actually writes – older
// sessions silently drop their output.
package logging

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
)

// Logger is the global logger. It is nil until Init is called.
var Logger *log.Logger

var (
	sessionFilePath string
	myPID           string
)

// Init initialises the logger. logPath is the path of the log file (e.g.
// "twchfetch.log"). The file is truncated on every call, so previous session
// output is wiped when a new session starts.
func Init(logPath string) error {
	myPID = strconv.Itoa(os.Getpid())
	sessionFilePath = logPath + ".session"

	// Claim ownership: write our PID to the session file.
	// Any older running instance will see this and stop writing.
	if err := os.WriteFile(sessionFilePath, []byte(myPID), 0644); err != nil {
		return fmt.Errorf("logging: write session file: %w", err)
	}

	// Truncate (or create) the log file for this session.
	f, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("logging: create log file: %w", err)
	}

	w := &sessionWriter{f: f}
	Logger = log.New(w)
	Logger.SetLevel(log.DebugLevel)
	Logger.SetReportTimestamp(true)
	Logger.SetTimeFormat("15:04:05")

	Logger.Info("Session started", "pid", myPID)
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

// ---------------------------------------------------------------------------
// sessionWriter — drops writes when this process is no longer the active
// session (i.e. a newer instance has claimed the session file).
// ---------------------------------------------------------------------------

type sessionWriter struct {
	f *os.File
}

func (w *sessionWriter) Write(p []byte) (int, error) {
	data, err := os.ReadFile(sessionFilePath)
	if err != nil || strings.TrimSpace(string(data)) != myPID {
		// Another session has taken ownership; silently discard.
		return len(p), nil
	}
	return w.f.Write(p)
}
