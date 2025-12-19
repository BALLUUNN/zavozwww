package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"social_service/internal/domain"
	"time"

	"github.com/google/uuid"
)

type UserClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewUserClient(baseURL string) *UserClient {
	return &UserClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *UserClient) UserExists(ctx context.Context, userID uuid.UUID) (bool, error) {
	url := fmt.Sprintf("%s/filmbuddy/exists", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("userid", userID.String())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result map[string]bool
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("failed to decode response: %w", err)
	}

	return result["exists"], nil
}

func (c *UserClient) GetUserByUsername(ctx context.Context, username string) (uuid.UUID, error) {
	url := fmt.Sprintf("%s/filmbuddy/username", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("username", username)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return uuid.Nil, fmt.Errorf("user not found")
	}

	if resp.StatusCode != http.StatusOK {
		return uuid.Nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return uuid.Nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return uuid.Parse(result.UserID)
}

func (c *UserClient) GetUserByID(ctx context.Context, userID uuid.UUID) (*domain.UserProfile, error) {
	url := fmt.Sprintf("%s/filmbuddy/profile", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("userid", userID.String())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var profile domain.UserProfile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &profile, nil
}
