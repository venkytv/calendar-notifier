package google

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
)

const (
	// OAuth2 scopes required for read-only calendar access
	calendarReadOnlyScope = calendar.CalendarReadonlyScope
)

// OAuth2Config holds the OAuth2 configuration
type OAuth2Config struct {
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	RedirectURIs []string `json:"redirect_uris"`
	AuthURI      string   `json:"auth_uri"`
	TokenURI     string   `json:"token_uri"`
}

// CredentialsFile represents the structure of the OAuth2 credentials file
type CredentialsFile struct {
	Installed OAuth2Config `json:"installed"`
	Web       OAuth2Config `json:"web"`
}

// TokenManager handles OAuth2 token management including refresh
type TokenManager struct {
	config    *oauth2.Config
	tokenFile string
	logger    *slog.Logger
}

// NewTokenManager creates a new token manager
func NewTokenManager(credentialsPath, tokenPath string, logger *slog.Logger) (*TokenManager, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Load OAuth2 credentials
	config, err := loadOAuth2Config(credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load OAuth2 config: %w", err)
	}

	return &TokenManager{
		config:    config,
		tokenFile: tokenPath,
		logger:    logger,
	}, nil
}

// loadOAuth2Config loads OAuth2 configuration from a credentials file
func loadOAuth2Config(credentialsPath string) (*oauth2.Config, error) {
	data, err := os.ReadFile(credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read credentials file: %w", err)
	}

	var creds CredentialsFile
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse credentials file: %w", err)
	}

	// Use "installed" config if available, otherwise "web"
	var oauthConfig OAuth2Config
	if creds.Installed.ClientID != "" {
		oauthConfig = creds.Installed
	} else if creds.Web.ClientID != "" {
		oauthConfig = creds.Web
	} else {
		return nil, fmt.Errorf("no valid OAuth2 configuration found in credentials file")
	}

	// Determine redirect URI
	redirectURI := "urn:ietf:wg:oauth:2.0:oob" // Default for desktop apps
	if len(oauthConfig.RedirectURIs) > 0 {
		redirectURI = oauthConfig.RedirectURIs[0]
	}

	return &oauth2.Config{
		ClientID:     oauthConfig.ClientID,
		ClientSecret: oauthConfig.ClientSecret,
		RedirectURL:  redirectURI,
		Scopes:       []string{calendarReadOnlyScope},
		Endpoint: oauth2.Endpoint{
			AuthURL:  oauthConfig.AuthURI,
			TokenURL: oauthConfig.TokenURI,
		},
	}, nil
}

// GetAuthURL generates the OAuth2 authorization URL for initial authentication
func (tm *TokenManager) GetAuthURL() string {
	// Use "offline" access type to get refresh token
	// Use prompt=consent to force showing the consent screen (to get refresh token)
	return tm.config.AuthCodeURL("state-token",
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"))
}

// ExchangeCode exchanges an authorization code for a token and saves it
func (tm *TokenManager) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := tm.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange authorization code: %w", err)
	}

	if err := tm.SaveToken(token); err != nil {
		return nil, fmt.Errorf("failed to save token: %w", err)
	}

	tm.logger.Info("successfully obtained and saved OAuth2 token")
	return token, nil
}

// LoadToken loads a saved token from disk
func (tm *TokenManager) LoadToken() (*oauth2.Token, error) {
	data, err := os.ReadFile(tm.tokenFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read token file: %w", err)
	}

	var token oauth2.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("failed to parse token file: %w", err)
	}

	return &token, nil
}

// SaveToken saves a token to disk
func (tm *TokenManager) SaveToken(token *oauth2.Token) error {
	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}

	if err := os.WriteFile(tm.tokenFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}

	return nil
}

// GetClient returns an HTTP client with a valid token, refreshing if necessary
func (tm *TokenManager) GetClient(ctx context.Context) (*http.Client, error) {
	token, err := tm.LoadToken()
	if err != nil {
		return nil, fmt.Errorf("failed to load token: %w (run initial authentication)", err)
	}

	// Create token source that automatically refreshes
	tokenSource := tm.config.TokenSource(ctx, token)

	// Get fresh token (will refresh if expired)
	freshToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to get valid token: %w", err)
	}

	// Save refreshed token if it changed
	if freshToken.AccessToken != token.AccessToken {
		tm.logger.Info("token refreshed, saving new token")
		if err := tm.SaveToken(freshToken); err != nil {
			tm.logger.Warn("failed to save refreshed token", "error", err)
		}
	}

	return oauth2.NewClient(ctx, tokenSource), nil
}

// IsTokenValid checks if a stored token exists and is valid
func (tm *TokenManager) IsTokenValid() bool {
	token, err := tm.LoadToken()
	if err != nil {
		return false
	}

	// Token is valid if it hasn't expired or has a refresh token
	return token.Valid() || token.RefreshToken != ""
}

// GetTokenExpiry returns the expiry time of the stored token
func (tm *TokenManager) GetTokenExpiry() (time.Time, error) {
	token, err := tm.LoadToken()
	if err != nil {
		return time.Time{}, err
	}
	return token.Expiry, nil
}
