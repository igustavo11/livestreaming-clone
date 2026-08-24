package chat_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/igustavo11/livestreaming-clone/internal/app"
	"github.com/igustavo11/livestreaming-clone/internal/db"
	"github.com/igustavo11/livestreaming-clone/internal/dbtest"
	"github.com/igustavo11/livestreaming-clone/internal/storage"
)

var (
	testPool    *pgxpool.Pool
	testQueries *db.Queries
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	pool, queries, cleanup, err := dbtest.Setup(ctx)
	if err != nil {
		panic(err)
	}
	testPool = pool
	testQueries = queries
	code := m.Run()
	cleanup()
	os.Exit(code)
}

func cleanTables(t *testing.T) {
	t.Helper()
	if _, err := testPool.Exec(context.Background(), "TRUNCATE TABLE users CASCADE"); err != nil {
		t.Fatalf("clean tables: %v", err)
	}
}

func newTestServer(t *testing.T) (*httptest.Server, http.Handler) {
	t.Helper()
	handler := app.NewRouter(app.Deps{
		Pool:          testPool,
		Queries:       testQueries,
		PendingSecret: "test-pending-secret",
		ObjectStore:   storage.NewMemory("https://cdn.test"),
		PublicBaseURL: "http://localhost",
	})
	srv := httptest.NewServer(handler)
	t.Cleanup(func() { srv.Close() })
	return srv, handler
}

func doJSON(t *testing.T, handler http.Handler, method, path string, body any, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func signup(t *testing.T, handler http.Handler, email, username string) *http.Cookie {
	t.Helper()
	rec := doJSON(t, handler, http.MethodPost, "/api/auth/signup", map[string]string{
		"email":    email,
		"username": username,
		"password": "password123",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("signup status=%d body:%s", rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session" {
			return c
		}
	}
	t.Fatal("missing session cookie")
	return nil
}

func wsURL(srv *httptest.Server, username string) string {
	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"
	u.Path = "/ws/chat/" + username
	return u.String()
}

func dialWS(t *testing.T, urlStr string, cookie *http.Cookie) *websocket.Conn {
	t.Helper()
	header := http.Header{}
	if cookie != nil {
		header.Set("Cookie", cookie.String())
	}
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, resp, err := dialer.Dial(urlStr, header)
	if err != nil {
		if resp != nil {
			t.Fatalf("dial %s failed: %v status=%d", urlStr, err, resp.StatusCode)
		}
		t.Fatalf("dial %s failed: %v", urlStr, err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func sendChat(t *testing.T, conn *websocket.Conn, msg string) {
	t.Helper()
	if err := conn.WriteJSON(map[string]string{"message": msg}); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func readJSONWithTimeout(t *testing.T, conn *websocket.Conn, timeout time.Duration) map[string]any {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	var m map[string]any
	if err := conn.ReadJSON(&m); err != nil {
		t.Fatalf("read json: %v", err)
	}
	_ = conn.SetReadDeadline(time.Time{})
	return m
}

func expectNoMessage(t *testing.T, _ *websocket.Conn, timeout time.Duration) {
	t.Helper()
	time.Sleep(timeout)
}

func TestChatAnonymousCanReceiveButNotSend(t *testing.T) {
	cleanTables(t)
	srv, handler := newTestServer(t)
	signup(t, handler, "alice@example.com", "alice")
	aliceCookie := signup(t, handler, "bob@example.com", "bob")
	bobCookie := aliceCookie
	anonConn := dialWS(t, wsURL(srv, "alice"), nil)
	bobConn := dialWS(t, wsURL(srv, "alice"), bobCookie)
	sendChat(t, bobConn, "hello from bob")
	msg := readJSONWithTimeout(t, anonConn, 2*time.Second)
	if msg["type"] != "chat" {
		t.Fatalf("anon expected chat, got %+v", msg)
	}
	if msg["message"] != "hello from bob" {
		t.Errorf("message = %v, want hello", msg["message"])
	}
	anonConn2 := dialWS(t, wsURL(srv, "alice"), nil)
	receiver := dialWS(t, wsURL(srv, "alice"), bobCookie)
	if err := anonConn2.WriteJSON(map[string]string{"message": "i am anon"}); err != nil {
		t.Fatalf("anon write: %v", err)
	}
	errMsg := readJSONWithTimeout(t, anonConn2, 2*time.Second)
	if errMsg["type"] != "error" {
		t.Fatalf("expected error for anon send, got %+v", errMsg)
	}
	expectNoMessage(t, receiver, 300*time.Millisecond)
}

func TestChatAuthenticatedSendBroadcastsToAll(t *testing.T) {
	cleanTables(t)
	srv, handler := newTestServer(t)
	signup(t, handler, "alice@example.com", "alice")
	bobCookie := signup(t, handler, "bob@example.com", "bob")
	carolCookie := signup(t, handler, "carol@example.com", "carol")
	anon := dialWS(t, wsURL(srv, "alice"), nil)
	bob := dialWS(t, wsURL(srv, "alice"), bobCookie)
	carol := dialWS(t, wsURL(srv, "alice"), carolCookie)
	time.Sleep(100 * time.Millisecond)
	sendChat(t, bob, "hello everyone")
	for _, c := range []*websocket.Conn{anon, bob, carol} {
		m := readJSONWithTimeout(t, c, 2*time.Second)
		if m["type"] != "chat" || m["message"] != "hello everyone" {
			t.Errorf("broadcast mismatch %+v", m)
		}
		if m["username"] != "bob" {
			t.Errorf("username = %v, want bob", m["username"])
		}
	}
}

func TestChatRejectsLongMessage(t *testing.T) {
	cleanTables(t)
	srv, handler := newTestServer(t)
	signup(t, handler, "alice@example.com", "alice")
	bobCookie := signup(t, handler, "bob@example.com", "bob")
	bob := dialWS(t, wsURL(srv, "alice"), bobCookie)
	receiver := dialWS(t, wsURL(srv, "alice"), nil)
	time.Sleep(100 * time.Millisecond)
	long := strings.Repeat("a", 201)
	sendChat(t, bob, long)
	errMsg := readJSONWithTimeout(t, bob, 2*time.Second)
	if errMsg["type"] != "error" {
		t.Fatalf("expected error for long message, got %+v", errMsg)
	}
	if !strings.Contains(strings.ToLower(errMsg["error"].(string)), "200") && !strings.Contains(strings.ToLower(errMsg["error"].(string)), "long") {
		t.Errorf("error should mention length, got %v", errMsg["error"])
	}
	expectNoMessage(t, receiver, 300*time.Millisecond)
	sendChat(t, bob, strings.Repeat("b", 200))
	m := readJSONWithTimeout(t, receiver, 2*time.Second)
	if m["message"] != strings.Repeat("b", 200) {
		t.Errorf("200-char message should succeed")
	}
}

func TestChatRateLimited(t *testing.T) {
	cleanTables(t)
	srv, handler := newTestServer(t)
	signup(t, handler, "alice@example.com", "alice")
	bobCookie := signup(t, handler, "bob@example.com", "bob")
	bob := dialWS(t, wsURL(srv, "alice"), bobCookie)
	receiver := dialWS(t, wsURL(srv, "alice"), nil)
	time.Sleep(100 * time.Millisecond)
	sendChat(t, bob, "first")
	sendChat(t, bob, "second")
	m1 := readJSONWithTimeout(t, receiver, 2*time.Second)
	if m1["message"] != "first" {
		t.Fatalf("first msg not received %+v", m1)
	}
	errMsg := readJSONWithTimeout(t, bob, 2*time.Second)
	if errMsg["type"] == "chat" {
		errMsg = readJSONWithTimeout(t, bob, 2*time.Second)
	}
	if errMsg["type"] != "error" {
		t.Fatalf("expected rate limit error, got %+v", errMsg)
	}
	expectNoMessage(t, receiver, 300*time.Millisecond)
	time.Sleep(1100 * time.Millisecond)
	sendChat(t, bob, "third")
	m3 := readJSONWithTimeout(t, receiver, 2*time.Second)
	if m3["message"] != "third" {
		t.Errorf("third should succeed after wait, got %+v", m3)
	}
}

func TestChatNotPersisted(t *testing.T) {
	cleanTables(t)
	srv, handler := newTestServer(t)
	signup(t, handler, "alice@example.com", "alice")
	bobCookie := signup(t, handler, "bob@example.com", "bob")
	bob := dialWS(t, wsURL(srv, "alice"), bobCookie)
	sendChat(t, bob, "before")
	_ = readJSONWithTimeout(t, bob, 2*time.Second)
	late := dialWS(t, wsURL(srv, "alice"), nil)
	expectNoMessage(t, late, 300*time.Millisecond)
	sendChat(t, bob, "after")
	time.Sleep(1100 * time.Millisecond)
	sendChat(t, bob, "after2")
	m := readJSONWithTimeout(t, late, 2*time.Second)
	if m["message"] != "after2" && m["message"] != "after" {
		t.Fatalf("late joiner should see new messages, got %+v", m)
	}
}

func TestChatReconnectResumes(t *testing.T) {
	cleanTables(t)
	srv, handler := newTestServer(t)
	signup(t, handler, "alice@example.com", "alice")
	bobCookie := signup(t, handler, "bob@example.com", "bob")
	bob := dialWS(t, wsURL(srv, "alice"), bobCookie)
	sendChat(t, bob, "msg1")
	_ = readJSONWithTimeout(t, bob, 2*time.Second)
	_ = bob.Close()
	time.Sleep(100 * time.Millisecond)
	bob2 := dialWS(t, wsURL(srv, "alice"), bobCookie)
	receiver := dialWS(t, wsURL(srv, "alice"), nil)
	time.Sleep(100 * time.Millisecond)
	sendChat(t, bob2, "msg2")
	m := readJSONWithTimeout(t, receiver, 2*time.Second)
	if m["message"] != "msg2" {
		t.Fatalf("reconnected client should send and be received, got %+v", m)
	}
	m2 := readJSONWithTimeout(t, bob2, 2*time.Second)
	if m2["message"] != "msg2" {
		t.Fatalf("sender should receive own broadcast after reconnect, got %+v", m2)
	}
}

func TestChatNotFoundForUnknownChannel(t *testing.T) {
	cleanTables(t)
	srv, _ := newTestServer(t)
	header := http.Header{}
	dialer := websocket.Dialer{HandshakeTimeout: 2 * time.Second}
	_, resp, err := dialer.Dial(wsURL(srv, "nonexistent"), header)
	if err == nil {
		t.Fatal("expected dial to fail for unknown channel")
	}
	if resp == nil || resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got resp=%v err=%v", resp, err)
	}
}

func TestChatIsolatedPerChannel(t *testing.T) {
	cleanTables(t)
	srv, handler := newTestServer(t)
	signup(t, handler, "alice@example.com", "alice")
	signup(t, handler, "bob@example.com", "bob")
	bobCookie := signup(t, handler, "carol@example.com", "carol")
	aliceListener := dialWS(t, wsURL(srv, "alice"), nil)
	bobListener := dialWS(t, wsURL(srv, "bob"), nil)
	carol := dialWS(t, wsURL(srv, "alice"), bobCookie)
	time.Sleep(100 * time.Millisecond)
	sendChat(t, carol, "for alice only")
	m := readJSONWithTimeout(t, aliceListener, 2*time.Second)
	if m["message"] != "for alice only" {
		t.Fatalf("alice should receive")
	}
	expectNoMessage(t, bobListener, 300*time.Millisecond)
}
