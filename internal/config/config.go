package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"
)

type Config struct {
	TelegramToken   string
	GitHubToken     string
	GitHubOwner     string
	GitHubRepo      string
	TelegramGroupID int64
	CheckCron       string
	AdminChatIDs    []int64
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		return nil, fmt.Errorf("error loading .env file: %w", err)
	}

	groupID, err := strconv.ParseInt(os.Getenv("TELEGRAM_GROUP_ID"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("TELEGRAM_GROUP_ID must be a valid integer: %w", err)
	}

	adminChatIDs, err := parseAdminChatIDs(os.Getenv("ADMIN_CHAT_IDS"))
	if err != nil {
		return nil, fmt.Errorf("ADMIN_CHAT_IDS is invalid: %w", err)
	}

	cfg := &Config{
		TelegramToken:   os.Getenv("TELEGRAM_TOKEN"),
		GitHubToken:     os.Getenv("GITHUB_TOKEN"),
		GitHubOwner:     os.Getenv("GITHUB_OWNER"),
		GitHubRepo:      os.Getenv("GITHUB_REPO"),
		TelegramGroupID: groupID,
		CheckCron:       os.Getenv("CHECK_CRON"),
		AdminChatIDs:    adminChatIDs,
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func parseAdminChatIDs(raw string) ([]int64, error) {
	var ids []int64
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid ID %q: %w", part, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (c *Config) validate() error {
	if c.TelegramToken == "" {
		return fmt.Errorf("TELEGRAM_TOKEN is required")
	}
	if c.GitHubToken == "" {
		return fmt.Errorf("GITHUB_TOKEN is required")
	}
	if c.GitHubOwner == "" {
		return fmt.Errorf("GITHUB_OWNER is required")
	}
	if c.GitHubRepo == "" {
		return fmt.Errorf("GITHUB_REPO is required")
	}
	if c.TelegramGroupID == 0 {
		return fmt.Errorf("TELEGRAM_GROUP_ID is required")
	}
	if c.CheckCron == "" {
		return fmt.Errorf("CHECK_CRON is required")
	}
	if _, err := cron.ParseStandard(c.CheckCron); err != nil {
		return fmt.Errorf("CHECK_CRON is invalid: %w", err)
	}
	if len(c.AdminChatIDs) == 0 {
		return fmt.Errorf("ADMIN_CHAT_IDS is required")
	}
	return nil
}
