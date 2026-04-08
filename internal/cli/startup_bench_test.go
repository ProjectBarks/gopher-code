package cli

import (
	"context"
	"testing"
	"time"

	"github.com/projectbarks/gopher-code/pkg/session"
)

// BenchmarkStartupTime measures the time to initialize the TUIv2.
// Target: <100ms for startup time (from program start to first render)
func BenchmarkStartupTime(b *testing.B) {
	config := session.DefaultConfig()
	state := session.New(config, "/home/user")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Note: This is just the setup time, not the full program run
		// The actual program would block on Run()
		_ = RunTUIV2(ctx, state, nil, false, "")
	}
}

// BenchmarkREPLStartupTime measures the time to initialize the old REPL.
// Target: <100ms for startup time
func BenchmarkREPLStartupTime(b *testing.B) {
	config := session.DefaultConfig()
	state := session.New(config, "/home/user")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Note: RunREPL blocks indefinitely, so we use a short timeout
		RunREPL(ctx, state, nil, nil, false, nil, false, "", nil)
	}
}
