package auth

import (
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestSaveAndReadToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "token.json")

	token := &oauth2.Token{
		AccessToken:  "test-access-token",
		TokenType:    "Bearer",
		RefreshToken: "test-refresh-token",
		Expiry:       time.Now().Add(time.Hour),
	}

	if err := SaveToken(path, token); err != nil {
		t.Fatalf("failed to save token: %s", err)
	}

	read, exists, err := ReadToken(path)
	if err != nil {
		t.Fatalf("failed to read token: %s", err)
	}
	if !exists {
		t.Fatal("expected token to exist after being saved")
	}
	if read.AccessToken != token.AccessToken || read.RefreshToken != token.RefreshToken {
		t.Errorf("expected round-tripped token %+v, got %+v", token, read)
	}
}

func TestReadTokenDoesNotExist(t *testing.T) {
	_, exists, err := ReadToken(filepath.Join(t.TempDir(), "no-such-token.json"))
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %s", err)
	}
	if exists {
		t.Fatal("expected token not to exist")
	}
}

func TestFileSourcePersistsToken(t *testing.T) {
	tokenPath := filepath.Join(t.TempDir(), "token.json")
	conf := getConfig("test-id", "test-secret", "")

	token := &oauth2.Token{
		AccessToken: "test-access-token",
		TokenType:   "Bearer",
		Expiry:      time.Now().Add(time.Hour),
	}

	source := FileSource(tokenPath, token, conf)
	returned, err := source.Token()
	if err != nil {
		t.Fatalf("failed to get token: %s", err)
	}
	if returned.AccessToken != token.AccessToken {
		t.Errorf("expected access token %q, got %q", token.AccessToken, returned.AccessToken)
	}

	// The token should now be persisted to disk
	if _, exists, _ := ReadToken(tokenPath); !exists {
		t.Fatal("expected token file to be created by file source")
	}
}
