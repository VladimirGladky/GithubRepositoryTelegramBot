package github

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/go-github/v83/github"
)

const (
	maxRetries = 3
	retryDelay = 2 * time.Second
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
	isCollab, _, err := c.client.Repositories.IsCollaborator(ctx, c.owner, c.repo, username)
	if err != nil {
		return fmt.Errorf("failed to check collaborator status: %w", err)
	}

	if isCollab {
		if _, err := c.client.Repositories.RemoveCollaborator(ctx, c.owner, c.repo, username); err != nil {
			return fmt.Errorf("failed to remove collaborator: %w", err)
		}
		return nil
	}

	return c.deletePendingInvitation(ctx, username)
}

func (c *Client) deletePendingInvitation(ctx context.Context, username string) error {
	opts := &github.ListOptions{PerPage: 100}
	for {
		invitations, resp, err := c.client.Repositories.ListInvitations(ctx, c.owner, c.repo, opts)
		if err != nil {
			return fmt.Errorf("failed to list invitations: %w", err)
		}

		for _, inv := range invitations {
			if inv.GetInvitee().GetLogin() == username {
				if _, err := c.client.Repositories.DeleteInvitation(ctx, c.owner, c.repo, inv.GetID()); err != nil {
					return fmt.Errorf("failed to delete invitation: %w", err)
				}
				return nil
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return nil
}

type tokenTransport struct {
	token string
}

func (t *tokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.token)

	var body []byte
	if req.Body != nil {
		b, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body.Close()
		body = b
	}

	var resp *http.Response
	var err error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if body != nil {
			req.Body = io.NopCloser(bytes.NewReader(body))
		}

		resp, err = http.DefaultTransport.RoundTrip(req)
		if err == nil && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}

		if resp != nil {
			resp.Body.Close()
		}

		if attempt < maxRetries {
			select {
			case <-time.After(retryDelay):
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
		}
	}

	return resp, err
}
