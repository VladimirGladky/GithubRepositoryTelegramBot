package storage

import (
	"GithubTelegramBot/internal/models"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type Storage struct {
	db *sql.DB
}

func New(path string) (*Storage, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping db: %w", err)
	}

	s := &Storage{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate db: %w", err)
	}

	return s, nil
}

func (s *Storage) migrate() error {
	query := `
	CREATE TABLE IF NOT EXISTS collaborators (
		telegram_id     INTEGER PRIMARY KEY,
		github_username TEXT NOT NULL,
		added_at        DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	_, err := s.db.Exec(query)
	return err
}

func (s *Storage) Save(telegramID int64, githubUsername string) error {
	query := `
	INSERT INTO collaborators (telegram_id, github_username)
	VALUES (?, ?)
	ON CONFLICT(telegram_id) DO UPDATE SET github_username = excluded.github_username`

	_, err := s.db.Exec(query, telegramID, githubUsername)
	if err != nil {
		return fmt.Errorf("failed to save collaborator: %w", err)
	}
	return nil
}

func (s *Storage) GetByTelegramID(telegramID int64) (*models.Collaborator, error) {
	var c models.Collaborator
	err := s.db.QueryRow(
		`SELECT telegram_id, github_username FROM collaborators WHERE telegram_id = ?`, telegramID,
	).Scan(&c.TelegramID, &c.GitHubUsername)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get collaborator: %w", err)
	}
	return &c, nil
}

func (s *Storage) GetAll() ([]models.Collaborator, error) {
	rows, err := s.db.Query(`SELECT telegram_id, github_username FROM collaborators`)
	if err != nil {
		return nil, fmt.Errorf("failed to get collaborators: %w", err)
	}
	defer rows.Close()

	var result []models.Collaborator
	for rows.Next() {
		var c models.Collaborator
		if err := rows.Scan(&c.TelegramID, &c.GitHubUsername); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, nil
}

func (s *Storage) Delete(telegramID int64) error {
	_, err := s.db.Exec(`DELETE FROM collaborators WHERE telegram_id = ?`, telegramID)
	if err != nil {
		return fmt.Errorf("failed to delete collaborator: %w", err)
	}
	return nil
}

func (s *Storage) Close() error {
	return s.db.Close()
}
