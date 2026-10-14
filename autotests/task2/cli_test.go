package task2

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"go-course-tests/fixtures"
	"go-course-tests/internal/harness"
	"go-course-tests/mockserver"
)

func run(t *testing.T, upstream string, args ...string) (string, error) {
	t.Helper()
	harness.RequireBinary(t)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	cmd := harness.Command(ctx, map[string]string{"MOVIE_BASE_URL": upstream}, args...)
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("CLI hung: %v\n%s", ctx.Err(), output)
	}
	return string(output), err
}

func expectMovie(t *testing.T, output string, id int) {
	t.Helper()
	m := fixtures.MovieAt(id)
	want := fmt.Sprintf("%d — %s — %d — %s", id, m.Title, m.Year, m.Director)
	count := 0
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == want {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected line exactly once: %q; found %d\n%s", want, count, output)
	}
}

func TestCourseFlags(t *testing.T) {
	harness.RequireBinary(t)
	for name, args := range map[string][]string{
		"missing_both": {}, "missing_from": {"--to=2"}, "missing_to": {"--from=1"},
		"reversed": {"--from=2", "--to=1"}, "workers_zero": {"--from=1", "--to=2", "--workers=0"},
		"workers_negative": {"--from=1", "--to=2", "--workers=-2"},
		"invalid_from":     {"--from=abc", "--to=2"}, "invalid_to": {"--from=1", "--to=abc"},
		"invalid_timeout": {"--from=1", "--to=2", "--timeout=abc"},
		"invalid_workers": {"--from=1", "--to=2", "--workers=abc"},
	} {
		t.Run(name, func(t *testing.T) {
			output, err := run(t, "http://127.0.0.1:1", args...)
			var exit *exec.ExitError
			if !errors.As(err, &exit) {
				t.Fatalf("expected nonzero exit, got %v\n%s", err, output)
			}
			if strings.TrimSpace(output) == "" {
				t.Fatal("expected an explanatory error message")
			}
		})
	}
}

func TestCourseDownloadRange(t *testing.T) {
	harness.RequireBinary(t)
	upstream := &mockserver.Server{LastID: 3}
	server := httptest.NewServer(upstream)
	defer server.Close()
	output, err := run(t, server.URL, "--from=1", "--to=3")
	if err != nil {
		t.Fatalf("valid range failed: %v\n%s", err, output)
	}
	for id := 1; id <= 3; id++ {
		expectMovie(t, output, id)
	}
	_, _, ids := upstream.Stats()
	seen := map[int]int{}
	for _, id := range ids {
		seen[id]++
	}
	if len(seen) != 3 || seen[1] != 1 || seen[2] != 1 || seen[3] != 1 {
		t.Fatalf("expected requests for ids 1,2,3 exactly once, got %v", ids)
	}
}

func TestCourseWorkerLimit(t *testing.T) {
	harness.RequireBinary(t)
	for _, workers := range []int{1, 3, 10} {
		t.Run(fmt.Sprint(workers), func(t *testing.T) {
			upstream := &mockserver.Server{LastID: 12, DefaultDelay: 60 * time.Millisecond}
			server := httptest.NewServer(upstream)
			defer server.Close()
			args := []string{"--from=1", "--to=12"}
			if workers != 10 {
				args = append(args, fmt.Sprintf("--workers=%d", workers))
			}
			output, err := run(t, server.URL, args...)
			if err != nil {
				t.Fatalf("CLI failed: %v\n%s", err, output)
			}
			for id := 1; id <= 12; id++ {
				expectMovie(t, output, id)
			}
			max, _, _ := upstream.Stats()
			if max > workers || max < 1 {
				t.Fatalf("workers=%d, maximum concurrent requests=%d", workers, max)
			}
			if workers > 1 && max < 2 {
				t.Fatal("requests were sequential despite multiple workers")
			}
		})
	}
}

func TestCourseIndividualErrors(t *testing.T) {
	harness.RequireBinary(t)
	for name, behavior := range map[string]mockserver.Behavior{
		"not_found": {Status: 404}, "rate_limit": {Status: 429}, "server_error": {Status: 500},
		"broken_json": {Body: "{invalid"}, "disconnect": {Disconnect: true},
	} {
		t.Run(name, func(t *testing.T) {
			upstream := &mockserver.Server{LastID: 3, ByID: map[int]mockserver.Behavior{2: behavior}}
			server := httptest.NewServer(upstream)
			defer server.Close()
			output, _ := run(t, server.URL, "--from=1", "--to=3", "--workers=1")
			expectMovie(t, output, 1)
			expectMovie(t, output, 3)
			found := false
			for _, line := range strings.Split(output, "\n") {
				if strings.Contains(line, "2") && strings.TrimSpace(line) != "2" && !strings.Contains(line, fixtures.MovieAt(1).Title) && !strings.Contains(line, fixtures.MovieAt(3).Title) {
					found = true
				}
			}
			if !found {
				t.Fatalf("expected an error message identifying movie 2\n%s", output)
			}
		})
	}
}

func TestCourseTimeout(t *testing.T) {
	harness.RequireBinary(t)
	upstream := &mockserver.Server{LastID: 2, ByID: map[int]mockserver.Behavior{1: {Delay: 2 * time.Second}}}
	server := httptest.NewServer(upstream)
	defer server.Close()
	output, _ := run(t, server.URL, "--from=1", "--to=2", "--workers=1", "--timeout=1")
	expectMovie(t, output, 2)
	_, canceled, _ := upstream.Stats()
	if canceled < 1 {
		t.Fatalf("slow request was not canceled by timeout\n%s", output)
	}
	if strings.Contains(output, fixtures.MovieAt(1).Title) {
		t.Fatalf("slow movie unexpectedly succeeded\n%s", output)
	}
	if !strings.Contains(output, "1") {
		t.Fatalf("expected an error identifying movie 1\n%s", output)
	}
}

func TestCourseInterrupt(t *testing.T) {
	harness.RequireBinary(t)
	upstream := &mockserver.Server{LastID: 20, DefaultDelay: 5 * time.Second}
	server := httptest.NewServer(upstream)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	cmd := harness.Command(ctx, map[string]string{"MOVIE_BASE_URL": server.URL}, "--from=1", "--to=20", "--workers=2", "--timeout=10")
	var output harness.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() { cancel(); <-done })
	deadline := time.Now().Add(time.Second)
	for {
		_, _, ids := upstream.Stats()
		if len(ids) >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("CLI never started requests\n%s", output.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		done <- err
		if exit, ok := err.(*exec.ExitError); ok && exit.ProcessState.ExitCode() == -1 {
			t.Fatalf("CLI was killed by SIGINT instead of handling it\n%s", output.String())
		}
	case <-time.After(time.Second):
		t.Fatalf("CLI did not stop promptly after SIGINT\n%s", output.String())
	}
	_, _, ids := upstream.Stats()
	if len(ids) >= 20 {
		t.Fatalf("all queued requests started after interrupt: %v", ids)
	}
}
