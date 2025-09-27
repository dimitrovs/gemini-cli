package auth

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google-gemini/gemini-cli-go/pkg/config"
	"golang.org/x/oauth2"
)

func TestNewAuthenticator_WithSavedToken(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "gemini-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	restore := config.SetUserHomeDirForTesting(tempDir, nil)
	defer restore()

	token := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		Expiry:       time.Now().Add(1 * time.Hour),
	}
	settings := &config.Settings{
		Security: &config.SecuritySettings{
			Auth: &config.AuthSettings{
				Token: token,
			},
		},
	}
	if err := config.SaveUserSettings(settings); err != nil {
		t.Fatalf("Failed to save user settings: %v", err)
	}

	auth, err := NewAuthenticator("oauth2")
	if err != nil {
		t.Fatalf("Expected no error from NewAuthenticator, but got %v", err)
	}

	oauth2Auth, ok := auth.(*OAuth2Authenticator)
	if !ok {
		t.Fatalf("Expected OAuth2Authenticator, but got %T", auth)
	}

	if oauth2Auth.token.AccessToken != token.AccessToken {
		t.Errorf("Expected access token to be '%s', but got '%s'", token.AccessToken, oauth2Auth.token.AccessToken)
	}
}

func TestOAuth2Authenticator_GetToken_RefreshToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"access_token": "new-access-token",
			"token_type": "Bearer",
			"refresh_token": "new-refresh-token",
			"expiry": "2099-01-01T00:00:00Z"
		}`))
	}))
	defer server.Close()

	tempDir, err := ioutil.TempDir("", "gemini-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	restore := config.SetUserHomeDirForTesting(tempDir, nil)
	defer restore()

	conf := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "http://localhost/auth",
			TokenURL: server.URL,
		},
	}

	initialToken := &oauth2.Token{
		AccessToken:  "expired-access-token",
		RefreshToken: "test-refresh-token",
		Expiry:       time.Now().Add(-1 * time.Hour),
	}

	auth := &OAuth2Authenticator{config: conf, token: initialToken}
	accessToken, err := auth.GetToken()
	if err != nil {
		t.Fatalf("Expected no error from GetToken, but got %v", err)
	}

	if accessToken != "new-access-token" {
		t.Errorf("Expected new access token, but got '%s'", accessToken)
	}

	// Verify that the new token was saved
	loadedSettings, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load settings: %v", err)
	}
	if loadedSettings.Security.Auth.Token.AccessToken != "new-access-token" {
		t.Errorf("Expected saved token to be updated, but it was not.")
	}
}

func TestOAuth2Authenticator_GetToken_NoToken(t *testing.T) {
	auth := &OAuth2Authenticator{
		config: &oauth2.Config{},
	}
	_, err := auth.GetToken()
	if err == nil {
		t.Fatal("Expected an error when getting token without authentication, but got nil")
	}
	expectedErrorPrefix := "failed to read authorization code"
	if !strings.HasPrefix(err.Error(), expectedErrorPrefix) {
		t.Errorf("Expected error to start with '%s', but got '%s'", expectedErrorPrefix, err.Error())
	}
}

func TestCloudShellAuthenticator_GetToken(t *testing.T) {
	// This test is limited because it cannot run in a real Cloud Shell environment.
	// It primarily checks that the code doesn't panic and returns an error as expected.
	auth := &CloudShellAuthenticator{}
	_, err := auth.GetToken()
	if err == nil {
		t.Errorf("Expected an error when running outside of Cloud Shell, but got nil")
	}
}