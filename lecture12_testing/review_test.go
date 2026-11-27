//go:build integration

// ЭТИ ТЕСТЫ НЕ ЗАПУСТЯТСЯ ТАК КАК ВЗЯТЫ
// ИЗ СУЩЕСТВУЮЩЕГО ПРОЕКТА
// И ПОМЕЩЕНЫ В ПРИМЕРЫ ДЛЯ ОЗНАКОМЛЕНИЯ
// С ИНТЕГРАЦИОННЫМИ ТЕСТАМИ И TESTCONTAINERS
package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ai-reviewer/internal/adapters/jobstore"
	"ai-reviewer/internal/adapters/storage"
	httpcontroller "ai-reviewer/internal/controller/http"
	"ai-reviewer/internal/domain"
	"ai-reviewer/internal/dto"
	"ai-reviewer/internal/usecase/review"
	"ai-reviewer/migrations"
	"ai-reviewer/pkg/utils"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/wait"
)

func postgresForTest(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	container, err := testcontainers.Run(ctx, "postgres:16-alpine",
		testcontainers.WithEnv(map[string]string{
			"POSTGRES_USER": "lecture", "POSTGRES_PASSWORD": "lecture", "POSTGRES_DB": "lecture",
		}),
		testcontainers.WithExposedPorts("5432/tcp"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2)),
	)
	if container != nil {
		t.Cleanup(func() {
			cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := container.Terminate(cleanup); err != nil {
				t.Errorf("terminate PostgreSQL: %v", err)
			}
		})
	}
	if err != nil {
		t.Fatalf("start PostgreSQL with Testcontainers: %v", err)
	}
	endpoint, err := container.PortEndpoint(ctx, "5432/tcp", "")
	if err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, "postgres://lecture:lecture@"+endpoint+"/lecture?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	migrator, err := migrations.New(pool, "../../migrations")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := migrator.Up(ctx); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	return pool
}

func TestSaveReviewUpsert(t *testing.T) {
	pool := postgresForTest(t)
	store := storage.NewDatabaseClient(pool)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	fixture := domain.ReviewComments{
		Owner: "example", Repo: "demo", PRNumber: 1, URL: "https://github.com/example/demo/pull/1",
		Mode: "comments", HwNo: 12, Title: "Lecture", HeadSHA: "abc", Rate: 1, MaxRate: 5, Describe: "First",
		Comments: []domain.ReviewComment{{
			FilePath: "main.go", Line: 1, Severity: "low", Title: "Edit", Body: "Details", RuleOrCategory: "style", HeadSHA: "abc",
		}},
	}
	if err := store.SaveReview(ctx, fixture); err != nil {
		t.Fatal(err)
	}
	fixture.Rate = 5
	fixture.Describe = "Updated"
	if err := store.SaveReview(ctx, fixture); err != nil {
		t.Fatal(err)
	}

	var reviews, comments int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM reviews").Scan(&reviews); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM comments").Scan(&comments); err != nil {
		t.Fatal(err)
	}
	if reviews != 1 || comments != 1 {
		t.Fatalf("rows = %d reviews, %d comments, want 1 each", reviews, comments)
	}

	var rate int
	var describe string
	if err := pool.QueryRow(ctx, "SELECT rate, describe FROM reviews").Scan(&rate, &describe); err != nil {
		t.Fatal(err)
	}
	if rate != 5 || describe != "Updated" {
		t.Errorf("stored review = %d %q, want 5 Updated", rate, describe)
	}
}

type fixtureGitHub struct{}

func (fixtureGitHub) FetchPr(context.Context, dto.FetchPrInputDto) (dto.FetchPrOutputDto, error) {
	return dto.FetchPrOutputDto{
		Title: "Lecture fixture", HeadSHA: "abc",
		Files: []dto.PrFileDto{{Path: "main.go", Status: "modified", Patch: "+package main"}},
	}, nil
}

type fixtureLLM struct{}

func (fixtureLLM) Review(_ context.Context, _ string, input string) (domain.ReviewResult, error) {
	if !strings.Contains(input, "+package main") {
		return domain.ReviewResult{}, fmt.Errorf("fixture patch missing from model input")
	}
	return domain.ReviewResult{
		Content: `{"rate":4,"max_rate":5,"describe":"Fixture result","comments":[]}`,
		Model:   "fixture",
	}, nil
}

func TestReviewHTTPToDatabase(t *testing.T) {
	pool := postgresForTest(t)
	store := storage.NewDatabaseClient(pool)
	pipeline := review.New(review.Dependencies{Fetcher: fixtureGitHub{}, Reviewer: fixtureLLM{}, Store: store},
		review.Options{Prompts: utils.Prompts{Modes: map[domain.Mode]string{domain.ModeComments: "Review the fixture"}}})
	jobs := review.NewJobs(t.Context(), pipeline, jobstore.NewJobRegistry(), store, review.JobsOptions{Workers: 1, QueueSize: 4})
	t.Cleanup(jobs.Stop)
	mux := http.NewServeMux()
	httpcontroller.NewHttpHandler(t.Context(), mux, jobs, "").Register()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	body := `{"mode":"comments","pull_requests":[{"url":"https://github.com/example/demo/pull/1","homework_number":12}]}`
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/review", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	var submitted dto.ReviewResponse
	decodeHTTP(t, server.Client(), request, &submitted)
	if len(submitted.IDs) != 1 {
		t.Fatalf("unexpected returned IDs: %q", submitted.IDs)
	}

	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/review?id="+submitted.IDs[0], nil)
		if err != nil {
			t.Fatal(err)
		}
		var job domain.JobInfo
		decodeHTTP(t, server.Client(), request, &job)
		if job.Status == domain.JobFailed {
			t.Fatalf("review job failed: %s", job.Err)
		}
		if job.Status == domain.JobDone {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("job did not reach done: last status=%s: %v", job.Status, ctx.Err())
		case <-ticker.C:
		}
	}

	var rate int
	var describe string
	if err := pool.QueryRow(ctx, "SELECT rate, describe FROM reviews WHERE owner=$1 AND repo=$2 AND hw_no=$3 AND head_sha=$4",
		"example", "demo", 12, "abc").Scan(&rate, &describe); err != nil {
		t.Fatal(err)
	}
	if rate != 4 || describe != "Fixture result" {
		t.Errorf("stored result = %d %q, want 4 Fixture result", rate, describe)
	}
}

func decodeHTTP(t *testing.T, client *http.Client, req *http.Request, dst any) {
	t.Helper()
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		t.Fatalf("%s %s: got %d, want 200: %s", req.Method, req.URL.Path, res.StatusCode, body)
	}
	if err := json.NewDecoder(res.Body).Decode(dst); err != nil {
		t.Fatal(err)
	}
}
