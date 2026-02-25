package telegrambot

import (
	githubclient "GithubTelegramBot/internal/github"
	"GithubTelegramBot/internal/storage"
	"GithubTelegramBot/pkg/logger"
	"context"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

type Bot struct {
	api          *tgbotapi.BotAPI
	github       *githubclient.Client
	storage      *storage.Storage
	ctx          context.Context
	groupID      int64
	checkCron    string
	adminChatIDs []int64
}

func New(token string, githubClient *githubclient.Client, storage *storage.Storage, ctx context.Context, groupID int64, checkCron string, adminChatIDs []int64) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	b := &Bot{
		api:          api,
		github:       githubClient,
		storage:      storage,
		ctx:          ctx,
		groupID:      groupID,
		checkCron:    checkCron,
		adminChatIDs: adminChatIDs,
	}

	b.log().Info("Bot initialized")

	return b, nil
}

func (b *Bot) log() *logger.Logger {
	return logger.GetLoggerFromCtx(b.ctx)
}

func (b *Bot) Run() {
	go b.runChecker()

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

func (b *Bot) runChecker() {
	defer func() {
		if r := recover(); r != nil {
			b.log().Error("Checker goroutine panicked, restarting...", zap.Any("recover", r))
			b.notifyAdmins(fmt.Sprintf("Checker goroutine panicked: %v\nRestarting...", r))
			go b.runChecker()
		}
	}()

	c := cron.New()

	_, err := c.AddFunc(b.checkCron, b.checkCollaborators)
	if err != nil {
		b.log().Error("Invalid cron expression", zap.String("cron", b.checkCron), zap.Error(err))
		b.notifyAdmins(fmt.Sprintf("ERROR: Invalid cron expression %q: %v", b.checkCron, err))
		return
	}

	b.log().Info("Checker started", zap.String("cron", b.checkCron))
	c.Start()

	<-b.ctx.Done()

	b.log().Info("Checker stopped")
	c.Stop()
}

func (b *Bot) checkCollaborators() {
	b.log().Info("Checking collaborators")

	collaborators, err := b.storage.GetAll()
	if err != nil {
		b.log().Error("Failed to get collaborators from db", zap.Error(err))
		b.notifyAdmins(fmt.Sprintf("ERROR: Failed to get collaborators from db: %v", err))
		return
	}

	var removed []string
	var failed []string

	for _, c := range collaborators {
		member, err := b.api.GetChatMember(tgbotapi.GetChatMemberConfig{
			ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
				ChatID: b.groupID,
				UserID: c.TelegramID,
			},
		})
		if err != nil {
			b.log().Error("Failed to check membership",
				zap.String("github_username", c.GitHubUsername),
				zap.Int64("telegram_id", c.TelegramID),
				zap.Error(err),
			)
			b.notifyAdmins(fmt.Sprintf("ERROR: Failed to check membership for @%s (tg_id=%d): %v", c.GitHubUsername, c.TelegramID, err))
			failed = append(failed, c.GitHubUsername)
			continue
		}

		if member.HasLeft() || member.WasKicked() {
			b.log().Info("User left the group, removing collaborator",
				zap.String("github_username", c.GitHubUsername),
			)

			if err := b.github.RemoveCollaborator(b.ctx, c.GitHubUsername); err != nil {
				b.log().Error("Failed to remove collaborator from github",
					zap.String("github_username", c.GitHubUsername),
					zap.Error(err),
				)
				b.notifyAdmins(fmt.Sprintf("ERROR: Failed to remove @%s from GitHub: %v", c.GitHubUsername, err))
				failed = append(failed, c.GitHubUsername)
				continue
			}

			if err := b.storage.Delete(c.TelegramID); err != nil {
				b.log().Error("Failed to delete collaborator from db",
					zap.String("github_username", c.GitHubUsername),
					zap.Error(err),
				)
				b.notifyAdmins(fmt.Sprintf("ERROR: Failed to delete @%s from db: %v", c.GitHubUsername, err))
				failed = append(failed, c.GitHubUsername)
				continue
			}

			b.log().Info("Collaborator removed successfully",
				zap.String("github_username", c.GitHubUsername),
			)
			removed = append(removed, c.GitHubUsername)
		}
	}

	b.log().Info("Collaborator check done",
		zap.Int("total", len(collaborators)),
		zap.Int("removed", len(removed)),
		zap.Int("failed", len(failed)),
	)

	report := fmt.Sprintf(
		"Collaborator check done\nTotal in DB: %d\nRemoved: %d\nFailed: %d",
		len(collaborators),
		len(removed),
		len(failed),
	)
	if len(removed) > 0 {
		report += fmt.Sprintf("\n\nRemoved users: %v", removed)
	}
	if len(failed) > 0 {
		report += fmt.Sprintf("\n\nFailed users: %v", failed)
	}

	b.notifyAdmins(report)
}
