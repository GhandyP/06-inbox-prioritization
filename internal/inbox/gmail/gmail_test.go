package gmail_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/GhandyP/06-inbox-prioritization/internal/inbox"
	"github.com/GhandyP/06-inbox-prioritization/internal/inbox/gmail"
)

// This package is the credential-bearing edge of the project, so its tests care
// less about the happy path than about the guards: one scope, no token inside a
// repository, and a refusal when the granted scope is wider than the one asked
// for.

var _ inbox.Reader = (*gmail.Client)(nil)

const readOnlyScope = "https://www.googleapis.com/auth/gmail.readonly"

func moduleRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}
	return root
}

func TestScopeIsTheReadOnlyScope(t *testing.T) {
	if gmail.Scope != readOnlyScope {
		t.Fatalf("want %q, got %q", readOnlyScope, gmail.Scope)
	}
}

// TestNoWriteScopeAppearsInTheModule guards the hard rule: the project never
// asks for a scope that could mutate a mailbox. The scanning test is skipped
// because its own pattern strings would otherwise match themselves.
func TestNoWriteScopeAppearsInTheModule(t *testing.T) {
	forbidden := []string{
		"auth/gmail.modify",
		"auth/gmail.compose",
		"auth/gmail.send",
		"auth/gmail.labels",
		"auth/gmail.settings",
		"auth/mail.google.com",
	}

	self, err := filepath.Abs("gmail_test.go")
	if err != nil {
		t.Fatalf("resolve test path: %v", err)
	}

	err = filepath.WalkDir(moduleRoot(t), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || path == self {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, bad := range forbidden {
			if strings.Contains(string(content), bad) {
				t.Errorf("%s mentions the write scope %q", path, bad)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk module: %v", err)
	}
}

func TestTokenPathInsideTheProjectRepositoryIsRefused(t *testing.T) {
	cfg := gmail.Config{
		TokenPath:   filepath.Join(moduleRoot(t), "token.json"),
		TokenSource: oauth2.StaticTokenSource(readOnlyToken()),
	}

	_, err := gmail.New(context.Background(), cfg)
	if err == nil {
		t.Fatal("a token inside the project repository must be refused")
	}
	if !strings.Contains(err.Error(), "token.json") {
		t.Fatalf("the refusal does not name the path: %v", err)
	}
}

func TestTokenPathOutsideTheProjectRepositoryIsAccepted(t *testing.T) {
	cfg := gmail.Config{
		TokenPath:   filepath.Join(t.TempDir(), "token.json"),
		TokenSource: oauth2.StaticTokenSource(readOnlyToken()),
		Endpoint:    fakeGmail(t, nil),
	}

	if _, err := gmail.New(context.Background(), cfg); err != nil {
		t.Fatalf("a token outside the project repository is fine: %v", err)
	}
}

// TestTokenPathInsideAnUnrelatedWorkingTreeIsAccepted pins the decision that
// narrowed the guard. A working tree that merely encloses the project, such as a
// home directory holding a .git marker left by a project-identity tool, must not
// reject every path under it.
func TestTokenPathInsideAnUnrelatedWorkingTreeIsAccepted(t *testing.T) {
	enclosing := t.TempDir()
	if err := os.Mkdir(filepath.Join(enclosing, ".git"), 0o750); err != nil {
		t.Fatalf("create the enclosing working tree: %v", err)
	}

	if err := gmail.CheckTokenPath(filepath.Join(enclosing, "token.json")); err != nil {
		t.Fatalf("an unrelated enclosing working tree must not be refused: %v", err)
	}
}

func TestDefaultTokenPathIsAbsoluteAndNamed(t *testing.T) {
	path, err := gmail.DefaultTokenPath()
	if err != nil {
		t.Fatalf("default token path: %v", err)
	}
	if !filepath.IsAbs(path) {
		t.Fatalf("want an absolute path, got %q", path)
	}
	if filepath.Base(path) != "token.json" {
		t.Fatalf("want a token.json leaf, got %q", filepath.Base(path))
	}
	if err := gmail.CheckTokenPath(path); err != nil {
		t.Fatalf("the default must survive the guard: %v", err)
	}
}

func TestNewRefusesATokenWithAWiderGrantedScope(t *testing.T) {
	token := readOnlyToken()
	token = token.WithExtra(map[string]any{
		"scope": readOnlyScope + " https://www.googleapis.com/auth/gmail.modify",
	})

	cfg := gmail.Config{
		TokenPath:   filepath.Join(t.TempDir(), "token.json"),
		TokenSource: oauth2.StaticTokenSource(token),
		Endpoint:    fakeGmail(t, nil),
	}

	_, err := gmail.New(context.Background(), cfg)
	if err == nil {
		t.Fatal("a token granted more than gmail.readonly must be refused")
	}
	if !strings.Contains(err.Error(), "modify") {
		t.Fatalf("the refusal does not name the extra scope: %v", err)
	}
}

func TestNewRequiresCredentialsWhenItHasToRunTheFlow(t *testing.T) {
	cfg := gmail.Config{
		CredentialsPath: filepath.Join(t.TempDir(), "absent-client-secret.json"),
		TokenPath:       filepath.Join(t.TempDir(), "token.json"),
	}

	if _, err := gmail.New(context.Background(), cfg); err == nil {
		t.Fatal("want an error when the client secret is missing")
	}
}

func TestMessagesMapsHeadersBodyAndDate(t *testing.T) {
	body := "Please confirm the amount before Friday."
	server := fakeGmail(t, map[string]string{
		"m1": messageJSON(t, "m1", "ana.ruiz@example.com", "Invoice 8841 due Friday",
			"multipart/alternative", body, 1760349120000),
	})

	client, err := gmail.New(context.Background(), gmail.Config{
		TokenPath:   filepath.Join(t.TempDir(), "token.json"),
		TokenSource: oauth2.StaticTokenSource(readOnlyToken()),
		Endpoint:    server,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	messages, err := client.Messages(context.Background())
	if err != nil {
		t.Fatalf("read messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("want 1 message, got %d", len(messages))
	}

	got := messages[0]
	if got.ID != "m1" {
		t.Errorf("id: got %q", got.ID)
	}
	if got.Sender != "ana.ruiz@example.com" {
		t.Errorf("sender: got %q", got.Sender)
	}
	if got.Subject != "Invoice 8841 due Friday" {
		t.Errorf("subject: got %q", got.Subject)
	}
	if got.Body != body {
		t.Errorf("body: got %q", got.Body)
	}
	if got.ReceivedAt.IsZero() {
		t.Error("received_at was not mapped from internalDate")
	}
}

func TestMessagesFallsBackToTheSnippet(t *testing.T) {
	snippet := "please confirm the amount"
	server := fakeGmail(t, map[string]string{
		"m1": messageJSON(t, "m1", "ana@example.com", "Invoice", "text/html", "", 1760349120000),
	})

	client, err := gmail.New(context.Background(), gmail.Config{
		TokenPath:   filepath.Join(t.TempDir(), "token.json"),
		TokenSource: oauth2.StaticTokenSource(readOnlyToken()),
		Endpoint:    server,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	messages, err := client.Messages(context.Background())
	if err != nil {
		t.Fatalf("read messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("want 1 message, got %d", len(messages))
	}
	if messages[0].Body == "" {
		t.Error("a message with no text/plain part must still carry something quotable")
	}
	if !strings.Contains(messages[0].Body, snippet) {
		t.Errorf("want the snippet as the fallback, got %q", messages[0].Body)
	}
}

func TestMessagesReturnsAnEmptyBatch(t *testing.T) {
	server := fakeGmail(t, map[string]string{})

	client, err := gmail.New(context.Background(), gmail.Config{
		TokenPath:   filepath.Join(t.TempDir(), "token.json"),
		TokenSource: oauth2.StaticTokenSource(readOnlyToken()),
		Endpoint:    server,
		Limit:       5,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	messages, err := client.Messages(context.Background())
	if err != nil {
		t.Fatalf("an empty mailbox is not an error: %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("want no messages, got %d", len(messages))
	}
}

func TestMessagesHonoursTheLimit(t *testing.T) {
	fixtures := map[string]string{}
	for _, id := range []string{"m1", "m2", "m3"} {
		fixtures[id] = messageJSON(t, id, "a@example.com", "subject "+id, "text/plain", "body", 1760349120000)
	}
	server := fakeGmail(t, fixtures)

	client, err := gmail.New(context.Background(), gmail.Config{
		TokenPath:   filepath.Join(t.TempDir(), "token.json"),
		TokenSource: oauth2.StaticTokenSource(readOnlyToken()),
		Endpoint:    server,
		Limit:       2,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	messages, err := client.Messages(context.Background())
	if err != nil {
		t.Fatalf("read messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("want 2 messages, got %d", len(messages))
	}
}

func readOnlyToken() *oauth2.Token {
	return (&oauth2.Token{
		AccessToken: "test-token",
		TokenType:   "Bearer",
		Expiry:      time.Now().Add(time.Hour),
	}).WithExtra(map[string]any{"scope": readOnlyScope})
}

func messageJSON(t *testing.T, id, from, subject, mimeType, body string, internalDate int64) string {
	t.Helper()
	payload := map[string]any{
		"mimeType": mimeType,
		"headers": []map[string]string{
			{"name": "From", "value": from},
			{"name": "Subject", "value": subject},
		},
	}
	// Gmail nests the readable text in a child part; the snippet is the only
	// text available when there is no text/plain part at all.
	if body != "" {
		payload["parts"] = []map[string]any{
			{
				"mimeType": "text/plain",
				"body": map[string]string{
					"data": base64.RawURLEncoding.EncodeToString([]byte(body)),
				},
			},
		}
	}
	raw, err := json.Marshal(map[string]any{
		"id":           id,
		"threadId":     "t-" + id,
		"internalDate": fmt.Sprint(internalDate),
		"snippet":      "please confirm the amount",
		"payload":      payload,
	})
	if err != nil {
		t.Fatalf("marshal fake message: %v", err)
	}
	return string(raw)
}

// fakeGmail serves a minimal read-only Gmail surface: list returns the fixture
// ids, and get returns the fixture body.
func fakeGmail(t *testing.T, fixtures map[string]string) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		w.Header().Set("Content-Type", "application/json")

		if strings.HasSuffix(path, "/messages") {
			ids := make([]string, 0, len(fixtures))
			for id := range fixtures {
				ids = append(ids, id)
			}
			items := make([]map[string]string, 0, len(ids))
			for _, id := range ids {
				items = append(items, map[string]string{"id": id, "threadId": "t-" + id})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"messages":           items,
				"resultSizeEstimate": len(items),
			})
			return
		}

		last := path[strings.LastIndex(path, "/")+1:]
		if body, ok := fixtures[last]; ok {
			_, _ = w.Write([]byte(body))
			return
		}
		http.NotFound(w, r)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server.URL
}
