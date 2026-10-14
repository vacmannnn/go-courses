// Minimal HTTP fixture; this is not a complete task3 solution (no gRPC).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type movie struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Year     int    `json:"year"`
	Director string `json:"director"`
}

func fetch(ctx context.Context, id int) (movie, int, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	url := fmt.Sprintf("%s/%d/info.0.json", strings.TrimRight(os.Getenv("MOVIE_BASE_URL"), "/"), id)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return movie{}, 0, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return movie{}, 0, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return movie{}, res.StatusCode, fmt.Errorf("upstream HTTP %d", res.StatusCode)
	}
	var m movie
	err = json.NewDecoder(res.Body).Decode(&m)
	return m, res.StatusCode, err
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /load-movies", func(w http.ResponseWriter, r *http.Request) {
		from, e1 := strconv.Atoi(r.URL.Query().Get("from"))
		to, e2 := strconv.Atoi(r.URL.Query().Get("to"))
		if e1 != nil || e2 != nil || from < 1 || to < from {
			http.Error(w, "invalid range", 400)
			return
		}
		movies := []movie{}
		for id := from; id <= to; id++ {
			m, _, err := fetch(r.Context(), id)
			if err != nil {
				http.Error(w, err.Error(), 502)
				return
			}
			movies = append(movies, m)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(movies)
	})
	mux.HandleFunc("GET /last-movie", func(w http.ResponseWriter, r *http.Request) {
		var last movie
		for id := 1; ; id++ {
			m, status, err := fetch(r.Context(), id)
			if status == 404 {
				break
			}
			if err != nil {
				http.Error(w, err.Error(), 502)
				return
			}
			last = m
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(last)
	})
	mux.HandleFunc("GET /swagger/index.html", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body>Swagger UI fixture</body></html>"))
	})
	if err := http.ListenAndServe(os.Getenv("HTTP_ADDR"), mux); err != nil {
		panic(err)
	}
}
