package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"github.com/google-gemini/gemini-cli-go/pkg/config"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	oauthClientID     = "681255809395-oo8ft2oprdrnp9e3aqf6av3hmdib135j.apps.googleusercontent.com"
	oauthClientSecret = "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"
)

// Authenticator is the interface for different authentication methods.
type Authenticator interface {
	Authenticate() error
	// GetToken returns the authentication token.
	GetToken() (string, error)
}

// NewAuthenticator returns a new authenticator based on the provided type.
func NewAuthenticator(authType string) (Authenticator, error) {
	settings, err := config.Load()
	if err != nil {
		settings = &config.Settings{}
	}

	var token *oauth2.Token
	if settings.Security != nil && settings.Security.Auth != nil {
		token = settings.Security.Auth.Token
	}

	switch authType {
	case "oauth2":
		conf := &oauth2.Config{
			ClientID:     oauthClientID,
			ClientSecret: oauthClientSecret,
			RedirectURL:  "urn:ietf:wg:oauth:2.0:oob",
			Scopes:       []string{"https://www.googleapis.com/auth/cloud-platform"},
			Endpoint:     google.Endpoint,
		}
		return &OAuth2Authenticator{config: conf, token: token}, nil
	case "cloud-shell":
		return &CloudShellAuthenticator{}, nil
	default:
		return nil, fmt.Errorf("unsupported authentication type: %s", authType)
	}
}

// OAuth2Authenticator handles OAuth2 authentication.
type OAuth2Authenticator struct {
	config *oauth2.Config
	token  *oauth2.Token
}

// Authenticate performs OAuth2 authentication.
func (a *OAuth2Authenticator) Authenticate() error {
	// Generate code verifier
	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		return fmt.Errorf("failed to generate random bytes for code verifier: %w", err)
	}
	codeVerifier := base64.RawURLEncoding.EncodeToString(verifierBytes)

	// Generate code challenge
	challengeBytes := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(challengeBytes[:])

	authURL := a.config.AuthCodeURL("state-token", oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
	fmt.Printf("Go to the following link in your browser:\n\n%s\n\n", authURL)
	fmt.Print("Enter verification code: ")

	var code string
	if _, err := fmt.Scan(&code); err != nil {
		return fmt.Errorf("failed to read authorization code: %w", err)
	}

	token, err := a.config.Exchange(context.Background(), code,
		oauth2.SetAuthURLParam("code_verifier", codeVerifier),
	)
	if err != nil {
		return fmt.Errorf("failed to exchange token: %w", err)
	}

	a.token = token
	if err := saveTokenToConfig(token); err != nil {
		return err
	}
	return nil
}

// GetToken returns the OAuth2 token.
func (a *OAuth2Authenticator) GetToken() (string, error) {
	if a.token == nil {
		if err := a.Authenticate(); err != nil {
			return "", err
		}
		return a.token.AccessToken, nil
	}

	tokenSource := a.config.TokenSource(context.Background(), a.token)
	newToken, err := tokenSource.Token()
	if err != nil {
		if err := a.Authenticate(); err != nil {
			return "", err
		}
		return a.token.AccessToken, nil
	}

	if newToken.AccessToken != a.token.AccessToken {
		a.token = newToken
		if err := saveTokenToConfig(newToken); err != nil {
			// Log this error, but we can still proceed.
		}
	}

	return a.token.AccessToken, nil
}

// CloudShellAuthenticator handles Cloud Shell authentication.
type CloudShellAuthenticator struct {
	token *oauth2.Token
}

// Authenticate performs Cloud Shell authentication.
func (a *CloudShellAuthenticator) Authenticate() error {
	creds, err := google.FindDefaultCredentials(context.Background(), "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return fmt.Errorf("failed to find default credentials: %w", err)
	}

	token, err := creds.TokenSource.Token()
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}

	a.token = token
	return nil
}

// GetToken returns the Cloud Shell token.
func (a *CloudShellAuthenticator) GetToken() (string, error) {
	if err := a.Authenticate(); err != nil {
		return "", err
	}
	if a.token == nil {
		return "", fmt.Errorf("authentication failed to produce a token")
	}
	return a.token.AccessToken, nil
}

func saveTokenToConfig(token *oauth2.Token) error {
	userSettings, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load user settings: %w", err)
	}
	if userSettings.Security == nil {
		userSettings.Security = &config.SecuritySettings{}
	}
	if userSettings.Security.Auth == nil {
		userSettings.Security.Auth = &config.AuthSettings{}
	}
	userSettings.Security.Auth.Token = token
	if err := config.SaveUserSettings(userSettings); err != nil {
		return fmt.Errorf("failed to save user settings with token: %w", err)
	}
	return nil
}