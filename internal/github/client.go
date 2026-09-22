package github

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-github/v83/github"
)

const (
	maxRetries = 3
	retryDelay = 2 * time.Second

	// rawErrorBodyHeader carries a copy of a 4xx response body from the
	// transport up to the caller. go-github drops the body when it cannot
	// unmarshal it (e.g. GitHub sometimes returns "errors" as a string
	// instead of an array), which would otherwise leave us with a bare
	// status code and no explanation.
	rawErrorBodyHeader = "X-Bot-Raw-Error-Body"
	maxRawErrorBody    = 2048
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
	// Do not send "permission": it is only valid for organization-owned
	// repositories. Since 2026-09-22 GitHub rejects it on personal repos with
	// 422 "Cannot assign <user> permission of read". Collaborators on a
	// personal private repo always get write access anyway.
	opts := &github.RepositoryAddCollaboratorOptions{}

	_, _, err := c.client.Repositories.AddCollaborator(ctx, c.owner, c.repo, username, opts)
	if err != nil {
		var ghErr *github.ErrorResponse
		if e, ok := err.(*github.ErrorResponse); ok {
			ghErr = e
		}
		if ghErr == nil {
			return fmt.Errorf("failed to add collaborator: %w", err)
		}

		raw := rawErrorBody(ghErr.Response)
		full := fmt.Errorf("failed to add collaborator: %w", err)
		if raw != "" {
			full = fmt.Errorf("failed to add collaborator: %w; body: %s", err, raw)
		}

		return &CollaboratorError{
			UserMessage: userMessage(ghErr, raw),
			FullError:   full,
		}
	}

	return nil
}

func userMessage(ghErr *github.ErrorResponse, raw string) string {
	if len(ghErr.Errors) > 0 && ghErr.Errors[0].Message != "" {
		return ghErr.Errors[0].Message
	}
	if ghErr.Message != "" {
		return ghErr.Message
	}
	if raw != "" {
		return raw
	}
	return fmt.Sprintf("GitHub вернул %d", ghErr.Response.StatusCode)
}

func rawErrorBody(resp *http.Response) string {
	if resp == nil {
		return ""
	}
	return strings.TrimSpace(resp.Header.Get(rawErrorBodyHeader))
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
			if resp.StatusCode >= 400 {
				stashErrorBody(resp)
			}
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

// stashErrorBody reads a 4xx body, puts it back for go-github to parse, and
// keeps a single-line copy in rawErrorBodyHeader.
func stashErrorBody(resp *http.Response) {
	if resp.Body == nil {
		return
	}
	b, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		resp.Body = io.NopCloser(bytes.NewReader(nil))
		return
	}
	resp.Body = io.NopCloser(bytes.NewReader(b))

	oneLine := strings.Join(strings.Fields(string(b)), " ")
	if len(oneLine) > maxRawErrorBody {
		oneLine = oneLine[:maxRawErrorBody] + "..."
	}
	resp.Header.Set(rawErrorBodyHeader, oneLine)
}
