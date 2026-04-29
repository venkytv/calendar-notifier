package google

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
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

// persistingRefresher is an oauth2.TokenSource that refreshes the access token
// using the stored refresh token and persists the result to disk. It is meant
// to be wrapped by oauth2.ReuseTokenSource, which handles caching and only
// calls this source when the cached token expires.
type persistingRefresher struct {
	mu           sync.Mutex
	config       *oauth2.Config
	ctx          context.Context
	refreshToken string
	manager      *TokenManager
}

func (r *persistingRefresher) Token() (*oauth2.Token, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Create a token source to perform the refresh. The token we pass has no
	// access token, so the underlying reuseTokenSource will immediately call
	// through to the HTTP token endpoint.
	src := r.config.TokenSource(r.ctx, &oauth2.Token{
		RefreshToken: r.refreshToken,
	})
	token, err := src.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	// Track refresh token rotation
	if token.RefreshToken != "" {
		r.refreshToken = token.RefreshToken
	}

	r.manager.logger.Info("token refreshed, persisting to disk")
	if err := r.manager.SaveToken(token); err != nil {
		r.manager.logger.Warn("failed to save refreshed token", "error", err)
	}

	return token, nil
}

// GetClient returns an HTTP client with a valid token, refreshing if necessary
func (tm *TokenManager) GetClient(ctx context.Context) (*http.Client, error) {
	token, err := tm.LoadToken()
	if err != nil {
		return nil, fmt.Errorf("failed to load token: %w (run initial authentication)", err)
	}

	refresher := &persistingRefresher{
		config:       tm.config,
		ctx:          ctx,
		refreshToken: token.RefreshToken,
		manager:      tm,
	}

	// ReuseTokenSource caches the token and only calls refresher.Token()
	// when the cached token expires.
	tokenSource := oauth2.ReuseTokenSource(token, refresher)

	// Validate that we can get a valid token (triggers a refresh if expired)
	if _, err := tokenSource.Token(); err != nil {
		return nil, fmt.Errorf("failed to get valid token: %w", err)
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
