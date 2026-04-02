package cli

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// Source: components/Spinner.tsx — spinners during API calls

// Spinner shows a simple animated spinner in the terminal.
type Spinner struct {
	mu      sync.Mutex
	frames  []string
	message string
	running bool
	done    chan struct{}
}

// SpinnerFrames is the default spinner animation.
var SpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// NewSpinner creates a spinner with a message.
func NewSpinner(message string) *Spinner {
	return &Spinner{
		frames:  SpinnerFrames,
		message: message,
		done:    make(chan struct{}),
	}
}

// Start begins the spinner animation.
func (s *Spinner) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	go func() {
		i := 0
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-s.done:
				// Clear the spinner line
				fmt.Fprintf(os.Stderr, "\r\033[K")
				return
			case <-ticker.C:
				s.mu.Lock()
				msg := s.message
				s.mu.Unlock()
				fmt.Fprintf(os.Stderr, "\r\033[36m%s\033[0m %s", s.frames[i%len(s.frames)], msg)
				i++
			}
		}
	}()
}

// Stop stops the spinner.
func (s *Spinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.running = false
	close(s.done)
}

// SetMessage updates the spinner message.
func (s *Spinner) SetMessage(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.message = msg
}
