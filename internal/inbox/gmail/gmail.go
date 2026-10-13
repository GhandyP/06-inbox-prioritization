// Package gmail reads messages from a Gmail mailbox.
//
// Read-only by construction. The only scope this package requests is
// gmail.readonly, and a test walks the module to make sure no wider scope string
// appears anywhere in it. There is no code path here that could archive, delete,
// reply to, label, or mark a message, because no scope that would permit one is
// ever requested.
package gmail

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gmailapi "google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"

	"github.com/GhandyP/06-inbox-prioritization/internal/priority"
)

// Scope is the only OAuth scope this package requests.
//
// Gmail read-only is the narrowest scope that can list and read messages. A
// wider scope would let a bug mutate a mailbox, so the constant is singular and
// no other scope string appears in this module.
const Scope = "https://www.googleapis.com/auth/gmail.readonly"

// defaultLimit caps how many messages one run reads.
const defaultLimit = 25

// Config configures the client.
type Config struct {
	// CredentialsPath is the OAuth client secret downloaded from Google Cloud,
	// for a client of type "Desktop app". It is only read when TokenSource is
	// nil.
	CredentialsPath string

	// TokenPath is where the cached token lives. It must sit outside any Git
	// working tree: a token inside one is a `git add -A` away from being
	// published.
	TokenPath string

	// Query is the Gmail search query. Empty reads the whole mailbox.
	Query string

	// Limit caps how many messages are read. Zero or less uses defaultLimit.
	Limit int

	// TokenSource overrides the installed-app flow. Tests use it to exercise
	// every path that does not need a browser; nil runs the real flow.
	TokenSource oauth2.TokenSource

	// Endpoint overrides the Gmail API endpoint. Tests point it at a local
	// server; empty uses the public one.
	Endpoint string
}

// Client reads messages through the Gmail API.
type Client struct {
	service *gmailapi.Service
	query   string
	limit   int
}

// New builds a read-only client.
//
// It refuses a token path inside a Git working tree and a token whose granted
// scope is wider than Scope, so a misconfigured run fails before it can read
// anything.
func New(ctx context.Context, cfg Config) (*Client, error) {
	tokenPath := cfg.TokenPath
	if strings.TrimSpace(tokenPath) == "" {
		var err error
		tokenPath, err = DefaultTokenPath()
		if err != nil {
			return nil, err
		}
	}
	if err := CheckTokenPath(tokenPath); err != nil {
		return nil, err
	}

	source := cfg.TokenSource
	if source == nil {
		var err error
		source, err = installedAppSource(ctx, cfg, tokenPath)
		if err != nil {
			return nil, err
		}
	}

	token, err := source.Token()
	if err != nil {
		return nil, fmt.Errorf("gmail: read the token: %w", err)
	}
	if err := checkGrantedScope(token); err != nil {
		return nil, err
	}

	options := []option.ClientOption{option.WithTokenSource(oauth2.StaticTokenSource(token))}
	if cfg.Endpoint != "" {
		options = append(options, option.WithEndpoint(cfg.Endpoint))
	}

	service, err := gmailapi.NewService(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("gmail: build the service: %w", err)
	}

	limit := cfg.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	return &Client{service: service, query: cfg.Query, limit: limit}, nil
}

// Messages reads the mailbox once and maps every message onto the contract.
func (c *Client) Messages(ctx context.Context) ([]priority.Message, error) {
	call := c.service.Users.Messages.List("me").Context(ctx)
	if c.query != "" {
		call = call.Q(c.query)
	}
	call = call.MaxResults(int64(c.limit))

	list, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("gmail: list messages: %w", err)
	}

	messages := make([]priority.Message, 0, len(list.Messages))
	for _, stub := range list.Messages {
		if len(messages) >= c.limit {
			break
		}
		full, err := c.service.Users.Messages.Get("me", stub.Id).
			Format("full").
			Context(ctx).
			Do()
		if err != nil {
			return nil, fmt.Errorf("gmail: read message %s: %w", stub.Id, err)
		}
		messages = append(messages, mapMessage(full))
	}
	return messages, nil
}

// DefaultTokenPath returns where the cached token lives when Config.TokenPath is
// empty: the user configuration directory, never a project directory.
func DefaultTokenPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("gmail: locate the user configuration directory: %w", err)
	}
	return filepath.Join(dir, "06-inbox-prioritization", "token.json"), nil
}

// CheckTokenPath refuses a token path that sits inside the repository the
// working directory belongs to.
//
// The walk starts at the working directory and stops at the first ancestor that
// holds a `.git` entry, so it finds the repository this project is committed to.
// A working tree that merely encloses the project, such as a home directory
// holding a `.git` marker left behind by a project-identity tool, is not refused:
// that rule would reject every path under $HOME while protecting nothing, because
// a marker repository has no history to publish. The limitation is deliberate,
// and `.gitignore` covers the token file as a second layer.
func CheckTokenPath(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("gmail: token path is empty")
	}

	root, ok := repositoryRoot()
	if !ok {
		return nil
	}

	absolute, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("gmail: resolve the token path: %w", err)
	}
	if !within(absolute, root) {
		return nil
	}

	return fmt.Errorf(
		"gmail: token path %s is inside the repository at %s; set Config.TokenPath to a location outside it",
		absolute, root)
}

// repositoryRoot returns the root of the repository the working directory
// belongs to, and false when the working directory is not inside one.
func repositoryRoot() (string, bool) {
	dir, err := os.Getwd()
	if err != nil {
		return "", false
	}
	for {
		// Lstat catches a linked worktree too, where .git is a file.
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// within reports whether path is root itself or sits underneath it.
func within(path, root string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return relative == "." || !strings.HasPrefix(relative, "..")
}

func mapMessage(message *gmailapi.Message) priority.Message {
	return priority.Message{
		ID:         message.Id,
		Sender:     header(message, "From"),
		Subject:    header(message, "Subject"),
		Body:       bodyOf(message),
		ReceivedAt: receivedAt(message),
	}
}

func header(message *gmailapi.Message, name string) string {
	if message.Payload == nil {
		return ""
	}
	for _, candidate := range message.Payload.Headers {
		if strings.EqualFold(candidate.Name, name) {
			return candidate.Value
		}
	}
	return ""
}

// bodyOf prefers the first text/plain part and falls back to the snippet, which
// Gmail always supplies. A quote has to appear in the text, so an empty body
// would make every reason for that message unverifiable.
func bodyOf(message *gmailapi.Message) string {
	if text := plainText(message.Payload); text != "" {
		return text
	}
	return message.Snippet
}

func plainText(part *gmailapi.MessagePart) string {
	if part == nil {
		return ""
	}
	if strings.EqualFold(part.MimeType, "text/plain") && part.Body != nil && part.Body.Data != "" {
		decoded, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(part.Body.Data)
		if err == nil {
			return string(decoded)
		}
	}
	for _, child := range part.Parts {
		if text := plainText(child); text != "" {
			return text
		}
	}
	return ""
}

// receivedAt uses internalDate, which Gmail sets from the message itself. It is
// never inferred from the body, because ordering depends on it.
func receivedAt(message *gmailapi.Message) time.Time {
	if message.InternalDate == 0 {
		return time.Time{}
	}
	return time.UnixMilli(message.InternalDate).UTC()
}

// checkGrantedScope refuses a token that carries more than this package asked
// for. The consent screen, not this code, decides what a token grants, so the
// grant is read back and checked rather than assumed.
func checkGrantedScope(token *oauth2.Token) error {
	granted := grantedScopes(token)
	if len(granted) == 0 {
		// The installed-app flow stores the granted scope next to the token;
		// a token without it cannot be checked, and an unchecked wider grant is
		// exactly the failure this guard exists to catch.
		return errors.New("gmail: the token does not record its granted scope, so it cannot be checked; delete it and authorize again")
	}
	for _, scope := range granted {
		if scope != Scope {
			return fmt.Errorf("gmail: the token grants %q, which is wider than %q", scope, Scope)
		}
	}
	return nil
}

func grantedScopes(token *oauth2.Token) []string {
	raw, ok := token.Extra("scope").(string)
	if !ok {
		return nil
	}
	return strings.Fields(raw)
}

// installedAppSource returns a token source backed by tokenPath, running the
// installed-app flow once when no usable token is cached.
func installedAppSource(ctx context.Context, cfg Config, tokenPath string) (oauth2.TokenSource, error) {
	if strings.TrimSpace(cfg.CredentialsPath) == "" {
		return nil, errors.New("gmail: Config.CredentialsPath is required when no TokenSource is supplied; download an OAuth client of type Desktop app from Google Cloud")
	}

	secret, err := os.ReadFile(cfg.CredentialsPath)
	if err != nil {
		return nil, fmt.Errorf("gmail: read the client secret %s: %w", cfg.CredentialsPath, err)
	}

	// ConfigFromJSON pins the requested scope to Scope and nothing else.
	config, err := google.ConfigFromJSON(secret, Scope)
	if err != nil {
		return nil, fmt.Errorf("gmail: parse the client secret: %w", err)
	}

	if token, err := readCachedToken(tokenPath); err == nil {
		return oauth2.ReuseTokenSource(token, config.TokenSource(ctx, token)), nil
	}

	token, err := authorizeWithBrowser(ctx, config)
	if err != nil {
		return nil, err
	}
	if err := writeCachedToken(tokenPath, token); err != nil {
		return nil, err
	}
	return oauth2.ReuseTokenSource(token, config.TokenSource(ctx, token)), nil
}

func readCachedToken(tokenPath string) (*oauth2.Token, error) {
	file, err := os.Open(tokenPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var token oauth2.Token
	if err := json.NewDecoder(file).Decode(&token); err != nil {
		return nil, fmt.Errorf("gmail: parse the cached token: %w", err)
	}
	return &token, nil
}

func writeCachedToken(tokenPath string, token *oauth2.Token) error {
	if err := os.MkdirAll(filepath.Dir(tokenPath), 0o700); err != nil {
		return fmt.Errorf("gmail: create the token directory: %w", err)
	}
	file, err := os.OpenFile(tokenPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("gmail: write the cached token: %w", err)
	}
	defer file.Close()

	return json.NewEncoder(file).Encode(token)
}

// authorizeWithBrowser runs the installed-app loopback flow: it listens on a
// local port, shows the consent URL, and exchanges the returned code.
//
// This is the one path in the package that the test suite cannot exercise,
// because it needs a real Google Cloud client and a browser. It is deliberately
// left thin so that as little unverified code as possible sits behind it.
func authorizeWithBrowser(ctx context.Context, config *oauth2.Config) (*oauth2.Token, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("gmail: listen for the authorization code: %w", err)
	}
	defer listener.Close()

	config.RedirectURL = "http://" + listener.Addr().String()

	code := make(chan string, 1)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		value := r.URL.Query().Get("code")
		if value == "" {
			http.Error(w, "authorization failed: no code in the callback", http.StatusBadRequest)
			return
		}
		fmt.Fprintln(w, "Authorization received. You can close this tab.")
		code <- value
	})}
	go func() { _ = server.Serve(listener) }()
	defer func() { _ = server.Shutdown(context.Background()) }()

	url := config.AuthCodeURL("state", oauth2.AccessTypeOffline)
	fmt.Printf("Open this URL to authorize read-only Gmail access:\n\n%s\n\n", url)
	_ = exec.Command("xdg-open", url).Start()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case value := <-code:
		token, err := config.Exchange(ctx, value)
		if err != nil {
			return nil, fmt.Errorf("gmail: exchange the authorization code: %w", err)
		}
		return token, nil
	}
}
