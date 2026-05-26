package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

var db *sql.DB

func initDB() error {
	var err error
	db, err = sql.Open("sqlite3", "./runner.db")
	if err != nil {
		return err
	}

	createUsersTable := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL
	);`
	if _, err := db.Exec(createUsersTable); err != nil {
		return err
	}

	createTokensTable := `
	CREATE TABLE IF NOT EXISTS refresh_tokens (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		token TEXT UNIQUE NOT NULL,
		user_id INTEGER NOT NULL,
		expires_at DATETIME NOT NULL,
		FOREIGN KEY(user_id) REFERENCES users(id)
	);`
	if _, err := db.Exec(createTokensTable); err != nil {
		return err
	}

	return nil
}

func ensureAdminUser(username, password string) error {
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username = ?)", username).Scan(&exists)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = db.Exec("INSERT INTO users (username, password_hash) VALUES (?, ?)", username, string(hash))
	return err
}

func verifyUser(username, password string) (int, error) {
	var id int
	var hash string
	err := db.QueryRow("SELECT id, password_hash FROM users WHERE username = ?", username).Scan(&id, &hash)
	if err != nil {
		return 0, fmt.Errorf("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return 0, fmt.Errorf("invalid credentials")
	}

	return id, nil
}

func generateRefreshToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func saveRefreshToken(userID int, token string, expiresInDays int) error {
	_, err := db.Exec("INSERT INTO refresh_tokens (token, user_id, expires_at) VALUES (?, ?, datetime('now', '+' || ? || ' days'))", token, userID, expiresInDays)
	return err
}

func verifyRefreshToken(token string) (int, error) {
	var userID int
	err := db.QueryRow("SELECT user_id FROM refresh_tokens WHERE token = ? AND expires_at > datetime('now')", token).Scan(&userID)
	if err != nil {
		return 0, fmt.Errorf("invalid or expired refresh token")
	}
	return userID, nil
}
