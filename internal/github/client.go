package github

import (
	"context"
	"fmt"
	"net/http"

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
	}

	return &Client{
		client: github.NewClient(httpClient),
		owner:  owner,
		repo:   repo,
	}
}

func (c *Client) AddCollaborator(ctx context.Context, username string) error {
	opts := &github.RepositoryAddCollaboratorOptions{
		Permission: "push",
	}

	resp, _, err := c.client.Repositories.AddCollaborator(ctx, c.owner, c.repo, username, opts)
	if err != nil {
		return fmt.Errorf("failed to add collaborator: %w", err)
	}

	// 201 — инвайт отправлен, 204 — уже является коллаборатором
	if resp != nil {
		_ = resp
	}

	return nil
}

// tokenTransport добавляет токен в заголовок каждого запроса
type tokenTransport struct {
	token string
}

func (t *tokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.token)
	return http.DefaultTransport.RoundTrip(req)
}
