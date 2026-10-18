package api_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bergercookie/cliparr/server/internal/api"
	"github.com/bergercookie/cliparr/server/internal/fakejellyfin"
)

type quickConnectStart struct {
	ID               string `json:"id"`
	Code             string `json:"code"`
	ExpiresInSeconds int    `json:"expiresInSeconds"`
}

type quickConnectPoll struct {
	Status string `json:"status"`
	Token  string `json:"token"`
	User   struct{ ID, Name string }
	Error  string `json:"error"`
}

// approve plays the part of a user's own Jellyfin app: it types the code in for the given user.
func (e *testEnv) approve(code, user string) {
	e.t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/QuickConnect/Authorize?code="+code, nil)
	req.Header.Set("Authorization", `MediaBrowser Token="token-`+user+`"`)
	rec := httptest.NewRecorder()
	e.fake.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		e.t.Fatalf("approve %s as %s: status %d", code, user, rec.Code)
	}
}

func (e *testEnv) startQuickConnect() quickConnectStart {
	e.t.Helper()
	var start quickConnectStart
	if code := e.call("POST", "/api/auth/quickconnect", "", nil, &start); code != 200 {
		e.t.Fatalf("start quick connect: status %d", code)
	}
	return start
}

func TestQuickConnectSignsInWithoutAPassword(t *testing.T) {
	e := newEnv(t)
	start := e.startQuickConnect()
	if len(start.Code) != 6 || start.ID == "" || start.ExpiresInSeconds <= 0 {
		t.Fatalf("unexpected start answer: %+v", start)
	}

	var poll quickConnectPoll
	if code := e.call("GET", "/api/auth/quickconnect/"+start.ID, "", nil, &poll); code != 200 || poll.Status != "pending" {
		t.Fatalf("before approval: %d %+v", code, poll)
	}
	if poll.Token != "" {
		t.Error("a pending sign-in must not hand out a token")
	}

	e.approve(start.Code, "alice")

	poll = quickConnectPoll{}
	if code := e.call("GET", "/api/auth/quickconnect/"+start.ID, "", nil, &poll); code != 200 || poll.Status != "approved" {
		t.Fatalf("after approval: %d %+v", code, poll)
	}
	if poll.Token == "" || poll.User.Name != "alice" {
		t.Fatalf("want alice with a Cliparr token, got %+v", poll)
	}
	var me struct{ Name string }
	if code := e.call("GET", "/api/me", poll.Token, nil, &me); code != 200 || me.Name != "alice" {
		t.Errorf("the token should work like a password login's: %d %+v", code, me)
	}

	if code := e.call("GET", "/api/auth/quickconnect/"+start.ID, "", nil, nil); code != http.StatusGone {
		t.Errorf("an approved sign-in works once, got %d the second time", code)
	}
}

func TestQuickConnectNeverShowsTheBrowserJellyfinsSecret(t *testing.T) {
	e := newEnv(t)
	req, _ := http.NewRequest(http.MethodPost, e.url+"/api/auth/quickconnect", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "secret") {
		t.Errorf("the answer must not carry Jellyfin's secret: %s", body)
	}
}

func TestQuickConnectSignInsStayApart(t *testing.T) {
	e := newEnv(t)
	forAlice, forBob := e.startQuickConnect(), e.startQuickConnect()
	e.approve(forBob.Code, "bob")
	e.approve(forAlice.Code, "alice")

	for id, want := range map[string]string{forAlice.ID: "alice", forBob.ID: "bob"} {
		var poll quickConnectPoll
		if code := e.call("GET", "/api/auth/quickconnect/"+id, "", nil, &poll); code != 200 || poll.User.Name != want {
			t.Errorf("sign-in for %s: %d %+v", want, code, poll)
		}
	}
}

func TestQuickConnectUnknownSignInHasExpired(t *testing.T) {
	e := newEnv(t)
	var body quickConnectPoll
	if code := e.call("GET", "/api/auth/quickconnect/not-an-id", "", nil, &body); code != http.StatusGone || body.Error == "" {
		t.Errorf("got %d %+v, want 410 with a message", code, body)
	}
}

func TestQuickConnectSwitchedOffInJellyfin(t *testing.T) {
	e := newEnvWithFake(t, fakejellyfin.New(fakejellyfin.WithoutQuickConnect()))

	var body quickConnectPoll
	if code := e.call("POST", "/api/auth/quickconnect", "", nil, &body); code != http.StatusConflict || !strings.Contains(body.Error, "turned off") {
		t.Errorf("got %d %+v, want 409 saying it is turned off", code, body)
	}
	var info struct{ QuickConnect, PasswordLogin bool }
	e.call("GET", "/api/info", "", nil, &info)
	if info.QuickConnect || !info.PasswordLogin {
		t.Errorf("info should offer the password only: %+v", info)
	}
}

func TestInfoTellsHowPeopleCanSignIn(t *testing.T) {
	e := newEnv(t)
	var info struct {
		Name, Version               string
		QuickConnect, PasswordLogin bool
	}
	if code := e.call("GET", "/api/info", "", nil, &info); code != 200 || info.Name != "cliparr" || info.Version != "test" {
		t.Fatalf("info: %d %+v", code, info)
	}
	if !info.QuickConnect || !info.PasswordLogin {
		t.Errorf("both ways should be on by default: %+v", info)
	}
}

func TestPasswordLoginCanBeTurnedOff(t *testing.T) {
	e := newEnv(t, func(o *api.Options) { o.DisablePasswordLogin = true })

	var body quickConnectPoll
	if code := e.call("POST", "/api/auth/login", "", map[string]string{"username": "alice", "password": "pw"}, &body); code != http.StatusForbidden || !strings.Contains(body.Error, "Quick Connect") {
		t.Errorf("password login: got %d %+v, want 403 pointing to Quick Connect", code, body)
	}
	var info struct{ PasswordLogin bool }
	e.call("GET", "/api/info", "", nil, &info)
	if info.PasswordLogin {
		t.Error("info should say that passwords are off")
	}

	start := e.startQuickConnect()
	e.approve(start.Code, "alice")
	var poll quickConnectPoll
	if code := e.call("GET", "/api/auth/quickconnect/"+start.ID, "", nil, &poll); code != 200 || poll.Status != "approved" {
		t.Errorf("Quick Connect should still work: %d %+v", code, poll)
	}
}

func TestQuickConnectLimitsHowManySignInsWait(t *testing.T) {
	e := newEnv(t)
	var last int
	for i := 0; i < 101; i++ {
		last = e.call("POST", "/api/auth/quickconnect", "", nil, nil)
		if i < 100 && last != 200 {
			t.Fatalf("sign-in %d: status %d", i+1, last)
		}
	}
	if last != http.StatusTooManyRequests {
		t.Errorf("the 101st waiting sign-in: got %d, want 429", last)
	}
}
