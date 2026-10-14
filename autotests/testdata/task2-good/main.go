// Reference fixture used to validate the grader itself.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"
)

func main() {
	from := flag.Int("from", 0, "first id")
	to := flag.Int("to", 0, "last id")
	workers := flag.Int("workers", 10, "worker count")
	timeout := flag.Duration("timeout", 5*time.Second, "request timeout")
	flag.Parse()
	if *from <= 0 || *to < *from || *workers <= 0 || *timeout <= 0 {
		fmt.Fprintln(os.Stderr, "invalid range, worker count or timeout")
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	base := strings.TrimRight(os.Getenv("MOVIE_BASE_URL"), "/")
	jobs := make(chan int)
	var wg sync.WaitGroup
	client := &http.Client{}
	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				if ctx.Err() != nil {
					return
				}
				func() {
					requestCtx, cancel := context.WithTimeout(ctx, *timeout)
					defer cancel()
					req, err := http.NewRequestWithContext(requestCtx, "GET", fmt.Sprintf("%s/%d/info.0.json", base, id), nil)
					if err != nil {
						fmt.Printf("%d: error: %v\n", id, err)
						return
					}
					res, err := client.Do(req)
					if err != nil {
						fmt.Printf("%d: error: %v\n", id, err)
						return
					}
					defer res.Body.Close()
					if res.StatusCode != 200 {
						fmt.Printf("%d: HTTP %d\n", id, res.StatusCode)
						return
					}
					var movie struct {
						Title    string
						Year     int
						Director string
					}
					if err := json.NewDecoder(res.Body).Decode(&movie); err != nil {
						fmt.Printf("%d: error: %v\n", id, err)
						return
					}
					fmt.Printf("%d — %s — %d — %s\n", id, movie.Title, movie.Year, movie.Director)
				}()
			}
		}()
	}
send:
	for id := *from; id <= *to; id++ {
		select {
		case <-ctx.Done():
			break send
		case jobs <- id:
		}
	}
	close(jobs)
	wg.Wait()
}
