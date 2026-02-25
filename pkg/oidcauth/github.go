package oidcauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

// IsGitHubActions returns true if running inside GitHub Actions with OIDC support.
func IsGitHubActions() bool {
	return os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL") != "" &&
		os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN") != ""
}

// RequestGitHubToken requests an OIDC identity token from the GitHub Actions
// runtime. The audience parameter is included in the token's aud claim.
func RequestGitHubToken(ctx context.Context, audience string) (string, error) {
	requestURL := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL")
	requestToken := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")

	if requestURL == "" || requestToken == "" {
		return "", fmt.Errorf("GitHub Actions OIDC environment variables not set")
	}

	u, err := url.Parse(requestURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse OIDC request URL: %w", err)
	}
	q := u.Query()
	q.Set("audience", audience)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "bearer "+requestToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to request OIDC token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OIDC token request failed (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse OIDC token response: %w", err)
	}

	if result.Value == "" {
		return "", fmt.Errorf("empty OIDC token in response")
	}

	return result.Value, nil
}
