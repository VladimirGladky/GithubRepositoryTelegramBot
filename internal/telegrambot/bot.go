package telegrambot

import (
	githubclient "GithubTelegramBot/internal/github"
	"GithubTelegramBot/pkg/logger"
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	api    *tgbotapi.BotAPI
	github *githubclient.Client
}

func New(token string, githubClient *githubclient.Client, ctx context.Context) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	logger.GetLoggerFromCtx(ctx).Info("Bot initialized")

	return &Bot{
		api:    api,
		github: githubClient,
	}, nil
}

func (b *Bot) Run() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}
		b.handleMessage(update.Message)
	}
}
