package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"GithubTelegramBot/internal/config"
	githubclient "GithubTelegramBot/internal/github"
	"GithubTelegramBot/internal/storage"
	"GithubTelegramBot/internal/telegrambot"
	"GithubTelegramBot/pkg/logger"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	ctx, err := logger.New(ctx)
	if err != nil {
		panic("failed to init logger: " + err.Error())
	}

	log := logger.GetLoggerFromCtx(ctx)

	cfg, err := config.Load()
	if err != nil {
		panic("Failed to load config: " + err.Error())
	}

	db, err := storage.New("/app/data/collaborators.db")
	if err != nil {
		log.Fatal("Failed to init storage")
	}
	defer db.Close()

	ghClient := githubclient.NewClient(cfg.GitHubToken, cfg.GitHubOwner, cfg.GitHubRepo)

	bot, err := telegrambot.New(cfg.TelegramToken, ghClient, db, ctx, cfg.TelegramGroupIDs, cfg.CheckCron, cfg.AdminChatIDs)
	if err != nil {
		log.Fatal("Failed to create bot")
	}

	log.Info("Bot is running...")
	bot.Run()
}
