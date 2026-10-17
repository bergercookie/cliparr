package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"
)

// Session links a Cliparr login to the Jellyfin token issued for that user.
type Session struct {
	UserID        string
	UserName      string
	JellyfinToken string
	ExpiresAt     time.Time
}

// CreateSession stores a session and returns the opaque token handed to the client.
// Only a hash of that token is stored, so a leaked database cannot be used to
// impersonate a Cliparr login (the Jellyfin token itself must be kept in clear
// text because Cliparr needs to present it to Jellyfin).
func (s *Store) CreateSession(ctx context.Context, sess Session) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO sessions (token_hash, user_id, user_name, jellyfin_token, expires_at) VALUES (?, ?, ?, ?, ?)",
		hashToken(token), sess.UserID, sess.UserName, sess.JellyfinToken, sess.ExpiresAt.Unix())
	if err != nil {
		return "", err
	}
	return token, nil
}

// GetSession looks up a non-expired session by its client token.
func (s *Store) GetSession(ctx context.Context, token string) (Session, error) {
	var sess Session
	var expires int64
	err := s.db.QueryRowContext(ctx,
		"SELECT user_id, user_name, jellyfin_token, expires_at FROM sessions WHERE token_hash = ? AND expires_at > ?",
		hashToken(token), time.Now().Unix()).Scan(&sess.UserID, &sess.UserName, &sess.JellyfinToken, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, err
	}
	sess.ExpiresAt = time.Unix(expires, 0)
	return sess, nil
}

// DeleteSession removes a session (logout).
func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash = ?", hashToken(token))
	return err
}

// DeleteExpiredSessions removes sessions past their expiry.
func (s *Store) DeleteExpiredSessions(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at <= ?", time.Now().Unix())
	return err
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
