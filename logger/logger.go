package logger

import (
	"log"
	"os"
)

type Logger interface {
	Log(format string, v ...any)
	Warn(format string, v ...any)
}

type nopLogger struct{}

func (nopLogger) Log(string, ...any)  {}
func (nopLogger) Warn(string, ...any) {}

// NewNopLogger returns a new logger whose methods are no-ops and don't log anything.
func NewNopLogger() Logger {
	return &nopLogger{}
}

type LeveledLogger struct {
	isVerbose bool

	Logger     *log.Logger
	WarnLogger *log.Logger
}

func NewLeveledLogger(verbose bool) *LeveledLogger {
	return &LeveledLogger{
		isVerbose:  verbose,
		Logger:     log.New(os.Stdout, "", log.LstdFlags),
		WarnLogger: log.New(os.Stderr, "", log.LstdFlags),
	}
}

func (l *LeveledLogger) Log(format string, v ...any) {
	log.Printf(format, v...)
}

func (l *LeveledLogger) Warn(format string, v ...any) {
	l.WarnLogger.Printf(format, v...)
}

func (l *LeveledLogger) Verbose() Logger {
	if !l.isVerbose {
		return NewNopLogger()
	}
	return l
}
