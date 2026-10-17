package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// Clip is a saved moment of a movie or episode.
type Clip struct {
	ID        int64     `json:"id"`
	UserID    string    `json:"-"`
	ItemID    string    `json:"itemId"`
	Title     string    `json:"title"`
	StartMs   int64     `json:"startMs"`
	EndMs     int64     `json:"endMs"`
	Quote     string    `json:"quote"`
	Note      string    `json:"note"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ClipFilter narrows ListClips. Empty fields are ignored.
type ClipFilter struct {
	Query string // matches title, quote or note
	Tag   string
}

const clipColumns = "id, user_id, item_id, title, start_ms, end_ms, quote, note, tags, created_at, updated_at"

// CreateClip stores a new clip for c.UserID and returns it with ID and timestamps set.
func (s *Store) CreateClip(ctx context.Context, c Clip) (Clip, error) {
	now := time.Now()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO clips (user_id, item_id, title, start_ms, end_ms, quote, note, tags, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.UserID, c.ItemID, c.Title, c.StartMs, c.EndMs, c.Quote, c.Note, encodeTags(c.Tags), now.Unix(), now.Unix())
	if err != nil {
		return Clip{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Clip{}, err
	}
	return s.GetClip(ctx, c.UserID, id)
}

// GetClip returns one clip owned by userID.
func (s *Store) GetClip(ctx context.Context, userID string, id int64) (Clip, error) {
	row := s.db.QueryRowContext(ctx, "SELECT "+clipColumns+" FROM clips WHERE id = ? AND user_id = ?", id, userID)
	c, err := scanClip(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Clip{}, ErrNotFound
	}
	return c, err
}

// ListClips returns the clips owned by userID, newest first.
func (s *Store) ListClips(ctx context.Context, userID string, f ClipFilter) ([]Clip, error) {
	query := "SELECT " + clipColumns + " FROM clips WHERE user_id = ?"
	args := []any{userID}
	if f.Query != "" {
		query += ` AND (title LIKE ? ESCAPE '\' OR quote LIKE ? ESCAPE '\' OR note LIKE ? ESCAPE '\')`
		like := "%" + escapeLike(f.Query) + "%"
		args = append(args, like, like, like)
	}
	if f.Tag != "" {
		query += ` AND tags LIKE ? ESCAPE '\'`
		args = append(args, "%,"+escapeLike(f.Tag)+",%")
	}
	query += " ORDER BY created_at DESC, id DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	clips := []Clip{}
	for rows.Next() {
		c, err := scanClip(rows)
		if err != nil {
			return nil, err
		}
		clips = append(clips, c)
	}
	return clips, rows.Err()
}

// UpdateClip overwrites the editable fields (range, quote, note, tags) of a clip.
func (s *Store) UpdateClip(ctx context.Context, c Clip) (Clip, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE clips SET start_ms = ?, end_ms = ?, quote = ?, note = ?, tags = ?, updated_at = ?
		 WHERE id = ? AND user_id = ?`,
		c.StartMs, c.EndMs, c.Quote, c.Note, encodeTags(c.Tags), time.Now().Unix(), c.ID, c.UserID)
	if err != nil {
		return Clip{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Clip{}, ErrNotFound
	}
	return s.GetClip(ctx, c.UserID, c.ID)
}

// DeleteClip removes a clip owned by userID.
func (s *Store) DeleteClip(ctx context.Context, userID string, id int64) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM clips WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

type scanner interface{ Scan(dest ...any) error }

func scanClip(row scanner) (Clip, error) {
	var c Clip
	var tags string
	var created, updated int64
	err := row.Scan(&c.ID, &c.UserID, &c.ItemID, &c.Title, &c.StartMs, &c.EndMs, &c.Quote, &c.Note, &tags, &created, &updated)
	if err != nil {
		return Clip{}, err
	}
	c.Tags = decodeTags(tags)
	c.CreatedAt = time.Unix(created, 0).UTC()
	c.UpdatedAt = time.Unix(updated, 0).UTC()
	return c, nil
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
