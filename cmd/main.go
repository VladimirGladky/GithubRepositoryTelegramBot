package main

import (
	"log"

	"GithubTelegramBot/internal/config"
	githubclient "GithubTelegramBot/internal/github"
	"GithubTelegramBot/internal/telegrambot"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	ghClient := githubclient.NewClient(cfg.GitHubToken, cfg.GitHubOwner, cfg.GitHubRepo)

	bot, err := telegrambot.New(cfg.TelegramToken, ghClient)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	log.Println("Bot is running...")
	bot.Run()
}
