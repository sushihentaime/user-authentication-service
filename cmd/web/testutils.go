package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	models "github.com/sushihentaime/user-management-service/internal/db"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var connURL string

func dbMigrate(dsn string) (*migrate.Migrate, error) {
	m, err := migrate.New("file://../../migrations", dsn)
	if err != nil {
		return nil, err
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return nil, err
	}

	return m, nil
}

func testDB(t *testing.T) *sql.DB {
	ctx := context.Background()

	container, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("docker.io/postgres:14.11-bookworm"),
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("user"),
		postgres.WithPassword("password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(5*time.Second)),
	)
	if err != nil {
		t.Fatalf("failed to start container: %s", err)
	}

	connURL, err = container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %s", err)
	}

	m, err := dbMigrate(connURL)
	if err != nil {
		t.Fatalf("could not run migrations: %v", err)
	}

	db := openDB(t)

	t.Cleanup(func() {
		fmt.Printf("Cleaning up\n")
		m.Drop()
	})

	return db
}

func openDB(t *testing.T) *sql.DB {
	db, err := sql.Open("postgres", connURL)
	if err != nil {
		t.Fatalf("could not open database: %v", err)
	}

	return db
}

func newTestApplication(t *testing.T) *application {
	db := testDB(t)

	cfg := config{
		Env: "testing",
	}

	return &application{
		config: cfg,
		logger: slog.New(slog.NewJSONHandler(io.Discard, nil)),
		models: models.NewModels(db),
	}
}

type testServer struct {
	*httptest.Server
}

func newTestServer(t *testing.T, h http.Handler) *testServer {
	ts := httptest.NewServer(h)

	t.Cleanup(ts.Close)

	return &testServer{ts}
}

func (ts *testServer) post(t *testing.T, path string, data any) (int, http.Header, envelope) {
	jsonPayload, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("could not marshal payload to JSON: %v", err)
	}

	body := bytes.NewReader(jsonPayload)
	res, err := ts.Client().Post(ts.URL+path, "application/json", body)
	if err != nil {
		t.Fatalf("could not send POST request: %v", err)
	}

	return readResponse(t, res)
}

func (ts *testServer) put(t *testing.T, path string, data any) (int, http.Header, envelope) {
	jsonPayload, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("could not marshal payload to JSON: %v", err)
	}

	body := bytes.NewReader(jsonPayload)
	req, err := http.NewRequest(http.MethodPut, ts.URL+path, body)
	if err != nil {
		t.Fatalf("could not create PUT request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("could not send PUT request: %v", err)
	}

	return readResponse(t, res)
}

func readResponse(t *testing.T, res *http.Response) (int, http.Header, envelope) {
	defer res.Body.Close()

	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("could not read response body: %v", err)
	}

	var envelope envelope
	err = json.Unmarshal(responseBody, &envelope)
	if err != nil {
		t.Fatalf("could not unmarshal JSON response: %v", err)
	}

	return res.StatusCode, res.Header, envelope
}

func cleanup(app *application) error {
	_, err := app.models.DB.Exec("DELETE FROM users")
	if err != nil {
		return err
	}

	_, err = app.models.DB.Exec("DELETE FROM tokens")
	if err != nil {
		return err
	}

	_, err = app.models.DB.Exec("DELETE FROM user_permissions")
	if err != nil {
		return err
	}

	fmt.Println("Cleaning up...")
	return nil
}

func strPtr(s string) *string {
	return &s
}
