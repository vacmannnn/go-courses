// Package mockserver implements the movie upstream, including controlled failures.
package mockserver

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-course-tests/fixtures"
)

type Behavior struct {
	Status     int
	Body       string
	Delay      time.Duration
	Disconnect bool
}

// Configure the server before starting requests. Counters are safe for concurrent reads.
type Server struct {
	LastID                      int
	DefaultStatus               int
	DefaultDelay                time.Duration
	ByID                        map[int]Behavior
	mu                          sync.Mutex
	active, maxActive, canceled int
	ids                         []int
}

func (s *Server) Stats() (maxActive, canceled int, ids []int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.maxActive, s.canceled, append([]int(nil), s.ids...)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if r.Method != http.MethodGet || len(parts) != 2 || parts[1] != "info.0.json" {
		http.NotFound(w, r)
		return
	}
	id, err := strconv.Atoi(parts[0])
	if err != nil || id < 1 {
		http.NotFound(w, r)
		return
	}
	s.mu.Lock()
	s.active++
	s.ids = append(s.ids, id)
	if s.active > s.maxActive {
		s.maxActive = s.active
	}
	s.mu.Unlock()
	defer func() { s.mu.Lock(); s.active--; s.mu.Unlock() }()
	b := s.ByID[id]
	delay := b.Delay
	if delay == 0 {
		delay = s.DefaultDelay
	}
	if delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-r.Context().Done():
			s.mu.Lock()
			s.canceled++
			s.mu.Unlock()
			return
		case <-timer.C:
		}
	}
	if b.Disconnect {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			_ = conn.Close()
		}
		return
	}
	status := b.Status
	if status == 0 {
		status = s.DefaultStatus
	}
	if status == 0 {
		status = http.StatusOK
		if id > s.LastID {
			status = http.StatusNotFound
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if b.Body != "" {
		_, _ = w.Write([]byte(b.Body))
		return
	}
	if status != http.StatusOK {
		_, _ = w.Write([]byte(`{"error":"upstream unavailable"}`))
		return
	}
	_ = json.NewEncoder(w).Encode(fixtures.MovieAt(id))
}
