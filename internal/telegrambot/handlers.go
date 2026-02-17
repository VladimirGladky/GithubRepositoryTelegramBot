package telegrambot

import (
	"context"
	"fmt"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (b *Bot) handleMessage(message *tgbotapi.Message) {
	username := strings.TrimSpace(message.Text)

	if username == "" {
		b.send(message.Chat.ID, "Отправь GitHub username пользователя.")
		return
	}

	username = strings.TrimPrefix(username, "@")

	log.Printf("Adding collaborator: %s", username)

	err := b.github.AddCollaborator(context.Background(), username)
	if err != nil {
		log.Printf("Error adding collaborator %s: %v", username, err)
		b.send(message.Chat.ID, fmt.Sprintf("Ошибка при добавлении @%s: %v", username, err))
		return
	}

	b.send(message.Chat.ID, fmt.Sprintf("Пользователю @%s отправлен инвайт в репозиторий.", username))
}

func (b *Bot) send(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Error sending message: %v", err)
	}
}
