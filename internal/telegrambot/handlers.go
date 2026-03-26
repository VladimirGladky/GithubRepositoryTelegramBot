package telegrambot

import (
	"errors"
	"fmt"
	"strings"

	githubclient "GithubTelegramBot/internal/github"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

func userMessageFromErr(err error) string {
	var collabErr *githubclient.CollaboratorError
	if errors.As(err, &collabErr) {
		return collabErr.UserMessage
	}
	return "произошла ошибка при выполнении операции"
}

func (b *Bot) handleMessage(message *tgbotapi.Message) {
	if !message.Chat.IsPrivate() {
		return
	}

	if message.IsCommand() {
		switch message.Command() {
		case "start":
			b.handleStart(message)
		default:
			b.send(message.Chat.ID, "Отправь свой GitHub username, чтобы получить доступ к репозиторию.")
		}
		return
	}

	if message.Text != "" {
		b.handleText(message)
	}
}

func (b *Bot) handleStart(message *tgbotapi.Message) {
	ok, err := b.isMember(message.From.ID)
	if err != nil {
		b.log().Error("Error checking membership", zap.Int64("user_id", message.From.ID), zap.Error(err))
		b.send(message.Chat.ID, "Не удалось проверить членство в группе. Попробуй позже.")
		return
	}
	if !ok {
		b.send(message.Chat.ID, "Как будто ты еще не вступил в наш замечательный Клуб АйТи Красавчиков.\n\nДля этого надо купить любой уровень подписки на https://boosty.to/itkrasavchik и добавиться в приватный телеграм чат клуба.\n\nПосле этого приходи обратно - вышлем тебе доступ к домашкам.\n\nОстались вопросы - пиши @itkrasavchik-у")
		return
	}
	b.send(message.Chat.ID, "Ты состоишь в группе!\n\nОтправь свой GitHub username, чтобы получить доступ к репозиторию.")
}

func (b *Bot) isMember(userID int64) (bool, error) {
	for _, groupID := range b.groupIDs {
		member, err := b.api.GetChatMember(tgbotapi.GetChatMemberConfig{
			ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
				ChatID: groupID,
				UserID: userID,
			},
		})

		if err != nil {
			continue
		}

		switch member.Status {
		case "member", "administrator", "creator", "restricted":
			return true, nil
		}
	}

	return false, nil
}

func (b *Bot) handleText(message *tgbotapi.Message) {
	username := strings.TrimSpace(message.Text)
	username = strings.TrimPrefix(username, "https://github.com/")
	username = strings.TrimPrefix(username, "http://github.com/")
	username = strings.TrimPrefix(username, "github.com/")
	username = strings.TrimPrefix(username, "@")
	username = strings.TrimSpace(username)

	if username == "" {
		b.send(message.Chat.ID, "Отправь свой GitHub username.")
		return
	}

	existing, err := b.storage.GetByTelegramID(message.From.ID)
	if err != nil {
		b.log().Error("Error checking existing collaborator", zap.Error(err))
		b.send(message.Chat.ID, "Произошла ошибка. Попробуй позже.")
		return
	}

	if existing != nil {
		b.send(message.Chat.ID, fmt.Sprintf("Ты уже зарегистрирован как @%s.", existing.GitHubUsername))
		return
	}

	ok, err := b.isMember(message.From.ID)
	if err != nil {
		b.log().Error("Error checking membership", zap.Int64("user_id", message.From.ID), zap.Error(err))
		b.send(message.Chat.ID, "Не удалось проверить членство в группе. Попробуй позже.")
		return
	}
	if !ok {
		b.send(message.Chat.ID, "Как будто ты еще не вступил в наш замечательный Клуб АйТи Красавчиков.\n\nДля этого надо купить любой уровень подписки на https://boosty.to/itkrasavchik и добавиться в приватный телеграм чат клуба.\n\nПосле этого приходи обратно - вышлем тебе доступ к домашкам.\n\nОстались вопросы - пиши @itkrasavchik-у")
		return
	}

	b.handleAdd(message, username)
}

func (b *Bot) handleAdd(message *tgbotapi.Message, username string) {
	b.log().Info("Adding collaborator", zap.String("username", username))

	if err := b.github.AddCollaborator(b.ctx, username); err != nil {
		b.log().Error("Error adding collaborator", zap.String("username", username), zap.Error(err))
		b.send(message.Chat.ID, fmt.Sprintf("Не удалось добавить @%s: %s", username, userMessageFromErr(err)))
		b.notifyAdmins(fmt.Sprintf("ERROR: Failed to add @%s as collaborator: %v", username, err))
		return
	}

	if err := b.storage.Save(message.From.ID, username); err != nil {
		b.log().Error("Error saving collaborator to db", zap.String("username", username), zap.Error(err))
		b.notifyAdmins(fmt.Sprintf("ERROR: Failed to save @%s to db: %v", username, err))
	}

	b.send(message.Chat.ID, fmt.Sprintf("Пользователю @%s отправлен инвайт в репозиторий. \n\n https://github.com/itkrasavchik/home-assignments", username))
}

func (b *Bot) send(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := b.api.Send(msg); err != nil {
		b.log().Error("Error sending message", zap.Int64("chat_id", chatID), zap.Error(err))
	}
}

func (b *Bot) notifyAdmins(text string) {
	for _, id := range b.adminChatIDs {
		b.send(id, text)
	}
}
