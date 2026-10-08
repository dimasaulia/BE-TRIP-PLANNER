package app_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/open-suite/boilerplate-golang/internal/app"
	"github.com/open-suite/boilerplate-golang/internal/platform/config"
	"github.com/open-suite/boilerplate-golang/internal/platform/database"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
)

const testOrigin = "http://localhost:3000"

var (
	server  *httptest.Server
	testApp *app.App
	testDB  *database.Database
	testCfg config.Config
	testLog *logger.Logger
)

// TestMain runs the suite against a real PostgreSQL given by TEST_DATABASE_URL
// (for example the docker compose database). It wipes the public schema.
func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		fmt.Println("TEST_DATABASE_URL is not set; skipping integration tests")
		os.Exit(0)
	}

	uploadDir, _ := os.MkdirTemp("", "trip-uploads-")
	logDir, _ := os.MkdirTemp("", "trip-logs-")
	defer os.RemoveAll(uploadDir)
	defer os.RemoveAll(logDir)

	for key, value := range map[string]string{
		"DATABASE_URL":                dsn,
		"APP_BASE_URL":                testOrigin,
		"DEV_LOGIN_ENABLED":           "true",
		"UPLOAD_DIR":                  uploadDir,
		"LOG_DIR":                     logDir,
		"LOG_LEVEL":                   "error",
		"MAX_UPLOAD_BYTES":            "200000",
		"RATE_LIMIT_AUTH_PER_MIN":     "100000",
		"RATE_LIMIT_INVITE_PER_MIN":   "100000",
		"RATE_LIMIT_UPLOAD_PER_MIN":   "100000",
		"RATE_LIMIT_SCHEDULE_PER_MIN": "100000",
		"TOKEN_ENC_KEY":               strings.Repeat("ab", 32),
		"GOOGLE_CLIENT_ID":            "",
	} {
		os.Setenv(key, value)
	}

	if err := resetDatabase(dsn); err != nil {
		fmt.Println("reset database:", err)
		os.Exit(1)
	}

	time.Local = time.UTC

	ctx := context.Background()
	var err error
	testApp, err = app.Initialize(ctx)
	if err != nil {
		fmt.Println("initialize:", err)
		os.Exit(1)
	}

	testCfg = config.Load()
	testLog, _ = logger.New(logger.Config{Level: "error", LogDir: logDir})
	testDB, err = database.New(ctx, testCfg, testLog)
	if err != nil {
		fmt.Println("database:", err)
		os.Exit(1)
	}

	server = httptest.NewServer(testApp.Handler())
	code := m.Run()

	server.Close()
	testApp.Shutdown()
	_ = testApp.Close()
	os.Exit(code)
}

func resetDatabase(dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.Exec(`DROP SCHEMA public CASCADE; CREATE SCHEMA public;`); err != nil {
		return err
	}
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(db, "../../migrations")
}

// client is one signed-in user with its own cookie jar and realtime client id.
type client struct {
	t        *testing.T
	http     *http.Client
	clientID string
	userID   string
	email    string
}

type response struct {
	Status int
	Header http.Header
	Raw    []byte
	Body   map[string]any
}

func newClient(t *testing.T) *client {
	t.Helper()

	jar, _ := cookiejar.New(nil)
	c := &client{
		t:        t,
		http:     &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		clientID: "c-" + uuid.NewString()[:8],
		email:    "user-" + uuid.NewString()[:8] + "@example.com",
	}

	res := c.do(http.MethodPost, "/auth/dev-login", map[string]any{"email": c.email, "name": "User " + c.email[5:9]})
	if res.Status != http.StatusOK {
		t.Fatalf("dev login failed: %d %s", res.Status, res.Raw)
	}

	me := c.do(http.MethodGet, "/me", nil)
	c.userID = me.str("data", "user", "id")
	return c
}

func (c *client) do(method string, path string, body any) *response {
	c.t.Helper()

	var reader io.Reader
	if body != nil {
		encoded, _ := json.Marshal(body)
		reader = bytes.NewReader(encoded)
	}
	return c.raw(method, path, reader, "application/json", testOrigin)
}

func (c *client) raw(method string, path string, body io.Reader, contentType string, origin string) *response {
	c.t.Helper()

	request, err := http.NewRequest(method, server.URL+"/api/v1"+path, body)
	if err != nil {
		c.t.Fatal(err)
	}
	if body != nil {
		request.Header.Set("Content-Type", contentType)
	}
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	request.Header.Set("X-Client-Id", c.clientID)

	result, err := c.http.Do(request)
	if err != nil {
		c.t.Fatal(err)
	}
	defer result.Body.Close()

	raw, _ := io.ReadAll(result.Body)
	parsed := map[string]any{}
	_ = json.Unmarshal(raw, &parsed)

	return &response{Status: result.StatusCode, Header: result.Header, Raw: raw, Body: parsed}
}

// get walks a decoded JSON document; it returns nil when a key is missing.
func get(value any, path ...string) any {
	for _, key := range path {
		var object map[string]any
		switch typed := value.(type) {
		case map[string]any:
			object = typed
		case message:
			object = typed
		default:
			return nil
		}
		value = object[key]
	}
	return value
}

func (r *response) str(path ...string) string {
	value, _ := get(r.Body, path...).(string)
	return value
}

func (r *response) num(path ...string) float64 {
	value, _ := get(r.Body, path...).(float64)
	return value
}

func (r *response) errCode() string {
	return r.str("error", "code")
}

func (r *response) items() []any {
	items, _ := get(r.Body, "data", "items").([]any)
	return items
}

func (r *response) expect(t *testing.T, status int) *response {
	t.Helper()
	if r.Status != status {
		t.Fatalf("expected status %d, got %d: %s", status, r.Status, r.Raw)
	}
	return r
}

func (r *response) expectError(t *testing.T, status int, code string) *response {
	t.Helper()
	if r.Status != status || r.errCode() != code {
		t.Fatalf("expected %d %s, got %d %s: %s", status, code, r.Status, r.errCode(), r.Raw)
	}
	return r
}

// newTrip creates a trip owned by c covering 2026-12-14..2026-12-16 in Jakarta.
func (c *client) newTrip() string {
	c.t.Helper()
	res := c.do(http.MethodPost, "/trips", map[string]any{
		"name":       "Bali",
		"start_date": "2026-12-14",
		"end_date":   "2026-12-16",
		"timezone":   "Asia/Jakarta",
	}).expect(c.t, http.StatusCreated)
	return res.str("data", "id")
}

// join makes other a member of the trip with the given role via an invite.
func (c *client) invite(tripID string, role string, other *client) {
	c.t.Helper()
	created := c.do(http.MethodPost, "/trips/"+tripID+"/invites", map[string]any{"role": role}).expect(c.t, http.StatusCreated)
	other.do(http.MethodPost, "/invites/"+created.str("data", "token")+"/accept", nil).expect(c.t, http.StatusOK)
}

func (c *client) newItem(tripID string, title string) string {
	c.t.Helper()
	return c.do(http.MethodPost, "/trips/"+tripID+"/library", map[string]any{
		"kind": "destination", "title": title, "description_md": "# " + title,
	}).expect(c.t, http.StatusCreated).str("data", "id")
}

func (c *client) newBlock(tripID string, start string, end string, title string) *response {
	c.t.Helper()
	return c.do(http.MethodPost, "/trips/"+tripID+"/schedule/blocks", map[string]any{
		"title": title, "start_at": start, "end_at": end,
	})
}
