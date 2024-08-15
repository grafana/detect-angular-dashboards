package logger

import (
	"log"
	"os"
)

type Logger interface {
	Log(format string, v ...any)
	Warn(format string, v ...any)
	Error(format string, v ...any)
	Errorf(format string, v ...any)
}

type nopLogger struct{}

func (nopLogger) Log(string, ...any)    {}
func (nopLogger) Warn(string, ...any)   {}
func (nopLogger) Error(string, ...any)  {}
func (nopLogger) Errorf(string, ...any) {}

// NewNopLogger returns a new logger whose methods are no-ops and don't log anything.
func NewNopLogger() Logger {
	return &nopLogger{}
}

type LeveledLogger struct {
	isVerbose bool

	Logger      *log.Logger
	WarnLogger  *log.Logger
	ErrorLogger *log.Logger
}

func NewLeveledLogger(verbose bool) *LeveledLogger {
	return &LeveledLogger{
		isVerbose:   verbose,
		Logger:      log.New(os.Stdout, "INFO: ", log.LstdFlags),
		WarnLogger:  log.New(os.Stderr, "WARN: ", log.LstdFlags),
		ErrorLogger: log.New(os.Stderr, "ERROR: ", log.LstdFlags),
	}
}

func (l *LeveledLogger) Log(format string, v ...any) {
	l.Logger.Printf(format, v...)
}

func (l *LeveledLogger) Warn(format string, v ...any) {
	l.WarnLogger.Printf(format, v...)
}

func (l *LeveledLogger) Error(format string, v ...any) {
	l.ErrorLogger.Printf(format, v...)
}

func (l *LeveledLogger) Errorf(format string, v ...any) {
	l.ErrorLogger.Printf(format, v...)
}

func (l *LeveledLogger) Verbose() Logger {
	if !l.isVerbose {
		return NewNopLogger()
	}
	return l
}
