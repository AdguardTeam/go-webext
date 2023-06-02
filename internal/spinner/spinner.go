// Package spinner helps to show a spinner while the application is working.
package spinner

import (
	"time"

	"github.com/theckman/yacspin"
)

// Spinner is a wrapper around yacspin.Spinner.
type Spinner struct {
	Spinner *yacspin.Spinner
}

// New creates a new spinner.
func New() *Spinner {
	const defaultMessage = "working"
	const defaultStopMessage = "done"
	const defaultStopFailMessage = "failed"

	yCfg := yacspin.Config{
		Frequency:         200 * time.Millisecond,
		CharSet:           yacspin.CharSets[51],
		Suffix:            " ",
		SuffixAutoColon:   true,
		ColorAll:          true,
		Message:           defaultMessage,
		StopCharacter:     "✓",
		StopColors:        []string{"fgGreen"},
		StopMessage:       defaultStopMessage,
		StopFailMessage:   defaultStopFailMessage,
		StopFailCharacter: "✗",
		StopFailColors:    []string{"fgRed"},
	}

	s, err := yacspin.New(yCfg)
	if err != nil {
		// we use panic because this error would be caught by tests
		panic(err)
	}

	return &Spinner{Spinner: s}
}

// Start starts the spinner.
func (s *Spinner) Start(msg string) {
	if msg != "" {
		s.Spinner.Message(msg)
	}
	err := s.Spinner.Start()
	if err != nil {
		// we use panic because this error would be caught by tests
		panic(err)
	}
}

// Stop stops the spinner.
func (s *Spinner) Stop(msg string) {
	if msg != "" {
		s.Spinner.StopMessage(msg)
	}
	err := s.Spinner.Stop()
	if err != nil {
		// we use panic because this error would be caught by tests
		panic(err)
	}
}

// StopFail stops the spinner with a fail message.
func (s *Spinner) StopFail(msg string) {
	if msg != "" {
		s.Spinner.StopFailMessage(msg)
	}
	err := s.Spinner.StopFail()
	if err != nil {
		// we use panic because this error would be visible in tests
		panic(err)
	}
}

// Message sets the spinner message.
func (s *Spinner) Message(msg string) {
	s.Spinner.Message(msg)
}
