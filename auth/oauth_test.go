package auth

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func newTestCallbackServer(t *testing.T) *callbackServer {
	t.Helper()

	cs, err := newCallbackServer()
	if err != nil {
		t.Fatalf("failed to start callback server: %s", err)
	}
	return cs
}

// getCallback performs a request against the callback server as a
// browser would do when redirected back after authorization.
// Request errors are ignored on purpose: on the error paths the
// callback server is closed as soon as the failure is signaled, which
// can cut the response short. Real failures are caught by the wait
// call in each test instead.
func getCallback(t *testing.T, cs *callbackServer, params url.Values) {
	t.Helper()

	res, err := http.Get(cs.redirectURL() + "?" + params.Encode())
	if err != nil {
		return
	}
	res.Body.Close()
}

func TestCallbackServerReceivesCode(t *testing.T) {
	cs := newTestCallbackServer(t)

	go getCallback(t, cs, url.Values{
		"state": {cs.state},
		"code":  {"test-auth-code"},
	})

	code, err := cs.wait(5 * time.Second)
	if err != nil {
		t.Fatalf("expected authorization code, got error: %s", err)
	}
	if code != "test-auth-code" {
		t.Errorf("expected code %q, got %q", "test-auth-code", code)
	}
}

func TestCallbackServerRejectsInvalidState(t *testing.T) {
	cs := newTestCallbackServer(t)

	go getCallback(t, cs, url.Values{
		"state": {"invalid-state"},
		"code":  {"test-auth-code"},
	})

	if _, err := cs.wait(5 * time.Second); err == nil {
		t.Fatal("expected error when state does not match, got none")
	}
}

func TestCallbackServerHandlesErrorParam(t *testing.T) {
	cs := newTestCallbackServer(t)

	go getCallback(t, cs, url.Values{
		"state": {cs.state},
		"error": {"access_denied"},
	})

	if _, err := cs.wait(5 * time.Second); err == nil {
		t.Fatal("expected error when error param is present, got none")
	}
}

func TestCallbackServerRejectsMissingCode(t *testing.T) {
	cs := newTestCallbackServer(t)

	go getCallback(t, cs, url.Values{
		"state": {cs.state},
	})

	if _, err := cs.wait(5 * time.Second); err == nil {
		t.Fatal("expected error when code param is missing, got none")
	}
}

func TestGetConfig(t *testing.T) {
	conf := getConfig("test-id", "test-secret", "http://127.0.0.1:12345/")

	// The endpoint urls must be the current Google OAuth endpoints
	if conf.Endpoint.TokenURL != "https://oauth2.googleapis.com/token" {
		t.Errorf("unexpected token url: %s", conf.Endpoint.TokenURL)
	}
	if conf.Endpoint.AuthURL != "https://accounts.google.com/o/oauth2/auth" {
		t.Errorf("unexpected auth url: %s", conf.Endpoint.AuthURL)
	}

	if conf.RedirectURL != "http://127.0.0.1:12345/" {
		t.Errorf("unexpected redirect url: %s", conf.RedirectURL)
	}

	if len(conf.Scopes) != 1 || conf.Scopes[0] != DriveScope {
		t.Errorf("expected scopes [%s], got %v", DriveScope, conf.Scopes)
	}

	// The generated authorization url must contain the loopback
	// redirect uri, an offline access type and forced consent
	authUrl := conf.AuthCodeURL("test-state", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	parsed, err := url.Parse(authUrl)
	if err != nil {
		t.Fatalf("failed to parse auth url: %s", err)
	}

	query := parsed.Query()
	if query.Get("redirect_uri") != "http://127.0.0.1:12345/" {
		t.Errorf("unexpected redirect_uri in auth url: %s", query.Get("redirect_uri"))
	}
	if query.Get("access_type") != "offline" {
		t.Errorf("unexpected access_type in auth url: %s", query.Get("access_type"))
	}
	if query.Get("prompt") != "consent" {
		t.Errorf("unexpected prompt in auth url: %s", query.Get("prompt"))
	}
	if query.Get("state") != "test-state" {
		t.Errorf("unexpected state in auth url: %s", query.Get("state"))
	}
}
