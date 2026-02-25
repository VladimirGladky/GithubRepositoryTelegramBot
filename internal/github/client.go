package github

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/go-github/v83/github"
)

type Client struct {
	client *github.Client
	owner  string
	repo   string
}

func NewClient(token, owner, repo string) *Client {
	httpClient := &http.Client{
		Transport: &tokenTransport{token: token},
		Timeout:   30 * time.Second,
	}

	return &Client{
		client: github.NewClient(httpClient),
		owner:  owner,
		repo:   repo,
	}
}

type CollaboratorError struct {
	UserMessage string
	FullError   error
}

func (e *CollaboratorError) Error() string {
	return e.FullError.Error()
}

func (c *Client) AddCollaborator(ctx context.Context, username string) error {
	opts := &github.RepositoryAddCollaboratorOptions{
		Permission: "pull",
	}

	resp, _, err := c.client.Repositories.AddCollaborator(ctx, c.owner, c.repo, username, opts)
	if err != nil {
		ghErr, ok := err.(*github.ErrorResponse)
		if ok && len(ghErr.Errors) > 0 {
			return &CollaboratorError{
				UserMessage: ghErr.Errors[0].Message,
				FullError:   fmt.Errorf("failed to add collaborator: %w", err),
			}
		}
		return fmt.Errorf("failed to add collaborator: %w", err)
	}

	if resp != nil {
		_ = resp
	}

	return nil
}

func (c *Client) RemoveCollaborator(ctx context.Context, username string) error {
	_, err := c.client.Repositories.RemoveCollaborator(ctx, c.owner, c.repo, username)
	if err != nil {
		return fmt.Errorf("failed to remove collaborator: %w", err)
	}
	return nil
}

type tokenTransport struct {
	token string
}

func (t *tokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.token)
	return http.DefaultTransport.RoundTrip(req)
}
