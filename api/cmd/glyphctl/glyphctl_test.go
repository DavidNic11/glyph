package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// captureStdout runs fn with os.Stdout redirected to a pipe and returns
// everything it wrote.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		data, _ := io.ReadAll(r)
		done <- string(data)
	}()
	fn()
	_ = w.Close()
	os.Stdout = old
	return <-done
}

func TestDoJSONSendsAuthAndHeaders(t *testing.T) {
	var gotAuth, gotXRW, gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotXRW = r.Header.Get("X-Requested-With")
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"11111111-1111-1111-1111-111111111111","title":"Hello"}]`))
	}))
	defer srv.Close()

	c := newClient(srv.URL, "tok-123")
	var out []map[string]any
	if err := c.doJSON("GET", "/tasks", nil, &out); err != nil {
		t.Fatalf("doJSON: %v", err)
	}
	if gotMethod != "GET" || gotPath != "/api/v1/tasks" {
		t.Errorf("unexpected request line: %s %s", gotMethod, gotPath)
	}
	if gotAuth != "Bearer tok-123" {
		t.Errorf("Authorization = %q, want Bearer tok-123", gotAuth)
	}
	if gotXRW != "glyphctl" {
		t.Errorf("X-Requested-With = %q, want glyphctl", gotXRW)
	}
	if len(out) != 1 || out[0]["title"] != "Hello" {
		t.Errorf("unexpected decode: %+v", out)
	}
}

func TestDoJSONParsesAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"forbidden","error_description":"token lacks scope"}`))
	}))
	defer srv.Close()

	c := newClient(srv.URL, "tok")
	err := c.doJSON("GET", "/pages", nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	ae, ok := err.(*apiError)
	if !ok {
		t.Fatalf("expected *apiError, got %T: %v", err, err)
	}
	if ae.Status != 403 || ae.Code != "forbidden" || ae.Msg != "token lacks scope" {
		t.Errorf("unexpected apiError: %+v", ae)
	}
	if !strings.Contains(err.Error(), "token lacks scope") {
		t.Errorf("error string missing detail: %s", err.Error())
	}
}

func TestAuthTokenFlow(t *testing.T) {
	var gotForm map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_ = r.ParseForm()
		gotForm = map[string]string{}
		for k := range r.PostForm {
			gotForm[k] = r.PostForm.Get(k)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"abc.def","token_type":"Bearer","expires_in":3600,"scope":"task:read"}`))
	}))
	defer srv.Close()

	out := captureStdout(t, func() {
		err := run([]string{
			"--url", srv.URL,
			"auth", "token",
			"--client-id", "cid", "--client-secret", "csecret",
			"--subject", "alice@test.com", "--org", "22222222-2222-2222-2222-222222222222",
			"--scope", "task:read",
		})
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	})

	if gotForm["grant_type"] != "client_credentials" {
		t.Errorf("grant_type = %q", gotForm["grant_type"])
	}
	if gotForm["client_id"] != "cid" || gotForm["client_secret"] != "csecret" {
		t.Errorf("client creds not sent: %+v", gotForm)
	}
	if gotForm["subject"] != "alice@test.com" || gotForm["org_id"] != "22222222-2222-2222-2222-222222222222" {
		t.Errorf("subject/org not sent: %+v", gotForm)
	}
	if strings.TrimSpace(out) != "abc.def" {
		t.Errorf("token output = %q, want abc.def", strings.TrimSpace(out))
	}
}

func TestRunTasksListTable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"33333333-3333-3333-3333-333333333333","title":"Write docs","status":"todo","priority":"high"}]`))
	}))
	defer srv.Close()

	out := captureStdout(t, func() {
		if err := run([]string{"--url", srv.URL, "--token", "t", "tasks", "list"}); err != nil {
			t.Fatalf("run: %v", err)
		}
	})
	if !strings.Contains(out, "Write docs") || !strings.Contains(out, "todo") {
		t.Errorf("table output missing expected content:\n%s", out)
	}
	if !strings.Contains(out, "STATUS") {
		t.Errorf("table header missing:\n%s", out)
	}
}

func TestRunTasksCreateSendsBody(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"44444444-4444-4444-4444-444444444444","title":"New"}`))
	}))
	defer srv.Close()

	out := captureStdout(t, func() {
		err := run([]string{
			"--url", srv.URL, "--token", "t", "--json",
			"tasks", "create", "--title", "New", "--priority", "low", "--tags", "a, b",
		})
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	})

	if body["title"] != "New" || body["priority"] != "low" {
		t.Errorf("unexpected body: %+v", body)
	}
	tags, ok := body["tags"].([]any)
	if !ok || len(tags) != 2 || tags[0] != "a" || tags[1] != "b" {
		t.Errorf("tags not split correctly: %+v", body["tags"])
	}
	if !strings.Contains(out, "44444444") {
		t.Errorf("output missing created id:\n%s", out)
	}
}

func TestRunRequiresTokenForAuthedCommands(t *testing.T) {
	err := run([]string{"--url", "http://localhost:9", "tasks", "list"})
	if err == nil || !strings.Contains(err.Error(), "no token configured") {
		t.Fatalf("expected token error, got %v", err)
	}
}

func TestHealth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	out := captureStdout(t, func() {
		if err := run([]string{"--url", srv.URL, "health"}); err != nil {
			t.Fatalf("run: %v", err)
		}
	})
	if !strings.Contains(out, "ok") {
		t.Errorf("health output = %q", out)
	}
}
