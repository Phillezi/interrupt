package nopr

import "github.com/go-logr/logr"

// NopLogger returns a no-op logr.Logger.
// It is used as a default logger for this library.
func NopLogger() logr.Logger {
	return logr.New(nopLogSink{})
}

// nopLogSink implements logr.LogSink and discards all log messages.
type nopLogSink struct{}

// Init is a no-op.
func (nopLogSink) Init(info logr.RuntimeInfo) {}

// Enabled always returns false to indicate that no log levels are enabled.
func (nopLogSink) Enabled(level int) bool { return false }

// Info is a no-op.
func (nopLogSink) Info(level int, msg string, keysAndValues ...any) {}

// Error is a no-op.
func (nopLogSink) Error(err error, msg string, keysAndValues ...any) {}

// WithValues returns itself since no state is stored.
func (n nopLogSink) WithValues(keysAndValues ...any) logr.LogSink { return n }

// WithName returns itself since no state is stored.
func (n nopLogSink) WithName(name string) logr.LogSink { return n }
