package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

func TestWriteTail(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		content string
		n       int
		want    string
	}{
		{"a\nb\nc\nd\n", 2, "c\nd\n"},
		{"a\nb\nc\nd", 2, "c\nd"},
		{"a\nb\n", 10, "a\nb\n"},
		{"a\nb\n", 0, ""},
		{"", 3, ""},
		{"single", 1, "single"},
	}
	for i, tc := range cases {
		path := filepath.Join(dir, "t.log")
		if err := os.WriteFile(path, []byte(tc.content), 0644); err != nil {
			t.Fatal(err)
		}
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		var sb strings.Builder
		offset, err := writeTail(&sb, f, tc.n)
		_ = f.Close()
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		if sb.String() != tc.want {
			t.Errorf("case %d: got %q, want %q", i, sb.String(), tc.want)
		}
		if offset != int64(len(tc.content)) {
			t.Errorf("case %d: offset %d, want %d", i, offset, len(tc.content))
		}
	}
}

func TestLogsStream(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	if err := os.WriteFile(path, []byte("l1\nl2\nl3\n"), 0644); err != nil {
		t.Fatal(err)
	}

	e := echo.New()
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/admin/logs/app.log?lines=2", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("name")
	c.SetParamValues("app.log")

	done := make(chan error, 1)
	go func() { done <- LogsStream(dir)(c) }()

	step := 3 * logsPollInterval
	time.Sleep(step)
	appendFile(t, path, "l4\n")
	time.Sleep(step)
	// Truncate: the stream must restart from the beginning of the file.
	if err := os.WriteFile(path, []byte("n1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(step)
	// Replace: remove and recreate, like `make clean` followed by new logs.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("r1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(step)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("stream did not stop after the request context was cancelled")
	}

	if got, want := rec.Body.String(), "l2\nl3\nl4\nn1\nr1\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if ct := rec.Header().Get(echo.HeaderContentType); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("content type %q", ct)
	}
}

func TestLogsStreamRejectsBadNames(t *testing.T) {
	dir := t.TempDir()
	e := echo.New()
	for _, name := range []string{"../main.json", "data.json", "missing.log"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("name")
		c.SetParamValues(name)
		if err := LogsStream(dir)(c); err != nil {
			t.Fatal(err)
		}
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", name, rec.Code)
		}
	}
}

func appendFile(t *testing.T, path, content string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if _, err = f.WriteString(content); err != nil {
		t.Fatal(err)
	}
}
