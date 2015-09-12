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
	// Job was cancelled or an fatal error ever happened.
	FatalError() error

	// Set fatal error code.
	SetFatalError(err error)

	// Get error count.
	Errors() int

	// Increase error count by 1.
	IncreaseErrors()

	// Write log message.
	Log(level int, format string, a ...interface{})
}

type updaterImpl struct {
	fatalError error // Fatal Error.
	errors     int   // Error count.
	verbose    bool  // Verbose mode.
}

func NewUpdater(verbose bool) Updater {
	return &updaterImpl{verbose: verbose}
}

func (me *updaterImpl) FatalError() error {
	return me.fatalError
}

func (me *updaterImpl) SetFatalError(fatalError error) {
	if me.fatalError == nil {
		me.fatalError = fatalError
	}
}

func (me *updaterImpl) Errors() int {
	return me.errors
}

// Increase error count by 1.
func (me *updaterImpl) IncreaseErrors() {
	me.errors++
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
