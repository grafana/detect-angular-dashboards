package logger

import "log"

type Logger struct {
	Verbose bool
}

func (Logger) Logf(format string, v ...any) {
	log.Printf(format, v...)
}

func (l Logger) Verbosef(format string, v ...any) {
	if !l.Verbose {
		return
	}
	l.Logf(format, v...)
}
