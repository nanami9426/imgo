package utils

import (
	"log"
	"os"
)

// Log provides a minimal logger for package-level use.
// It is intentionally small to keep dependencies light.
var Log = NewLogger()

type Logger struct {
	std *log.Logger
}

func NewLogger() *Logger {
	return &Logger{
		std: log.New(os.Stderr, "[imgo] ", log.LstdFlags|log.Lshortfile),
	}
}

func (l *Logger) Errorf(format string, args ...any) {
	if l == nil || l.std == nil {
		return
	}
	l.std.Printf("ERROR: "+format, args...)
}
