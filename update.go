// File deduplication
package main

import (
	"fmt"
	"os"
)

const (
	LOG_TRACE = iota
	LOG_INFO
	LOG_WARN
	LOG_ERROR
)

// Update status.
type Updater interface {
	// Job was cancelled or an error ever happened.
	Error() error

	// Set error code.
	SetError(err error)

	// Write log message.
	Log(level int, format string, a ...interface{})
}

type updaterImpl struct {
	err     error // Error.
	verbose bool  // Verbose mode.
}

func NewUpdater(verbose bool) Updater {
	return &updaterImpl{verbose: verbose}
}

func (me *updaterImpl) Error() error {
	return me.err
}

func (me *updaterImpl) SetError(err error) {
	if me.err == nil {
		me.err = err
	}
}

func getLevelPrefix(level int) string {
	switch level {
	case LOG_TRACE:
		return ""

	case LOG_INFO:
		return ""

	case LOG_WARN:
		return "<W> "

	case LOG_ERROR:
		return "<E> "
	}

	// Never run here.
	return ""
}

func (me *updaterImpl) Log(level int, format string, a ...interface{}) {
	if level == LOG_TRACE && !me.verbose {
		return
	}

	if level == LOG_ERROR {
		fmt.Fprintf(os.Stderr, getLevelPrefix(level)+format+"\n", a...)
	} else {
		fmt.Fprintf(os.Stdout, getLevelPrefix(level)+format+"\n", a...)
	}
}
