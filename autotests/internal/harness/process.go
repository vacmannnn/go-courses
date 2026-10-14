// Package harness manages student executables for black-box acceptance tests.
package harness

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

type Buffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *Buffer) Write(p []byte) (int, error) { b.mu.Lock(); defer b.mu.Unlock(); return b.b.Write(p) }
func (b *Buffer) String() string              { b.mu.Lock(); defer b.mu.Unlock(); return b.b.String() }

func Command(ctx context.Context, env map[string]string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, os.Getenv("COURSE_SOLUTION_BIN"), args...)
	cmd.Env = os.Environ()
	for key, value := range env {
		prefix := key + "="
		filtered := cmd.Env[:0]
		for _, item := range cmd.Env {
			if !strings.HasPrefix(item, prefix) {
				filtered = append(filtered, item)
			}
		}
		cmd.Env = append(filtered, prefix+value)
	}
	return cmd
}

func RequireBinary(t *testing.T) {
	t.Helper()
	if os.Getenv("COURSE_SOLUTION_BIN") == "" {
		t.Skip("run through scripts/check.sh with a student solution")
	}
}

// StartHTTP uses the documented HTTP_ADDR and MOVIE_BASE_URL contract.
func StartHTTP(t *testing.T, upstream string) string {
	t.Helper()
	RequireBinary(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	cmd := Command(ctx, map[string]string{"HTTP_ADDR": addr, "MOVIE_BASE_URL": upstream})
	var logs Buffer
	cmd.Stdout, cmd.Stderr = &logs, &logs
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() { cancel(); <-done })
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			// Restore the completion for cleanup.
			done <- err
			t.Fatalf("HTTP service exited before becoming ready: %v\n%s", err, logs.String())
		case <-deadline.C:
			t.Fatalf("service did not listen on HTTP_ADDR=%s\n%s", addr, logs.String())
		case <-ticker.C:
			conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
			if err == nil {
				_ = conn.Close()
				return fmt.Sprintf("http://%s", addr)
			}
		}
	}
}
