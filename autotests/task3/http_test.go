package task3

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go-course-tests/fixtures"
	"go-course-tests/internal/harness"
	"go-course-tests/mockserver"
)

func request(t *testing.T, url string) (int, []byte) {
	t.Helper()
	client := &http.Client{Timeout: 3 * time.Second}
	res, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, body
}

func service(t *testing.T, upstream *mockserver.Server) string {
	t.Helper()
	harness.RequireBinary(t)
	server := httptest.NewServer(upstream)
	t.Cleanup(server.Close)
	return harness.StartHTTP(t, server.URL)
}

func TestCourseLoadMovies(t *testing.T) {
	base := service(t, &mockserver.Server{LastID: 3})
	status, body := request(t, base+"/load-movies?from=1&to=3")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", status, body)
	}
	var movies []fixtures.Movie
	if err := json.Unmarshal(body, &movies); err != nil {
		t.Fatalf("expected JSON array: %v; %s", err, body)
	}
	if len(movies) != 3 {
		t.Fatalf("expected 3 movies, got %d: %s", len(movies), body)
	}
	seen := map[int]bool{}
	for _, movie := range movies {
		if movie.ID < 1 || movie.ID > 3 || seen[movie.ID] {
			t.Fatalf("unexpected or duplicate id: %d", movie.ID)
		}
		seen[movie.ID] = true
		want := fixtures.MovieAt(movie.ID)
		if movie.Title != want.Title || movie.Year != want.Year || movie.Director != want.Director {
			t.Fatalf("movie %d: got %+v, want %+v", movie.ID, movie, want)
		}
	}
}

func TestCourseInvalidRange(t *testing.T) {
	base := service(t, &mockserver.Server{LastID: 3})
	for _, query := range []string{"from=3&to=1", "from=hello&to=3", "from=1&to=hello", "to=3", "from=1", ""} {
		t.Run(query, func(t *testing.T) {
			status, body := request(t, base+"/load-movies?"+query)
			if status != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", status, body)
			}
			if len(strings.TrimSpace(string(body))) == 0 {
				t.Fatal("expected an error message")
			}
		})
	}
}

func TestCourseLastMovie(t *testing.T) {
	base := service(t, &mockserver.Server{LastID: 3})
	status, body := request(t, base+"/last-movie")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", status, body)
	}
	var movie fixtures.Movie
	if err := json.Unmarshal(body, &movie); err != nil {
		t.Fatalf("expected movie JSON: %v", err)
	}
	want := fixtures.MovieAt(3)
	if movie.ID != want.ID || movie.Title != want.Title || movie.Year != want.Year || movie.Director != want.Director {
		t.Fatalf("expected last movie %+v, got %+v", want, movie)
	}
}

func TestCourseSwagger(t *testing.T) {
	base := service(t, &mockserver.Server{LastID: 3})
	status, body := request(t, base+"/swagger/index.html")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", status, body)
	}
	html := strings.ToLower(string(body))
	if !strings.Contains(html, "swagger") || !strings.Contains(html, "<html") {
		t.Fatalf("expected Swagger HTML page, got %.200s", body)
	}
}

func TestCourseUpstreamFailure(t *testing.T) {
	// /last-movie avoids the unspecified partial-range response contract.
	base := service(t, &mockserver.Server{LastID: 3, DefaultStatus: 500})
	status, body := request(t, base+"/last-movie")
	if status < 400 || status >= 600 {
		t.Fatalf("upstream 500 must become an HTTP error, got %d: %s", status, body)
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		t.Fatal("expected an explanatory error body")
	}
	// A failed upstream request must not crash the entire HTTP service.
	status, _ = request(t, base+"/load-movies?from=bad&to=3")
	if status != http.StatusBadRequest {
		t.Fatalf("service should remain responsive after upstream failure, got %d", status)
	}
}
