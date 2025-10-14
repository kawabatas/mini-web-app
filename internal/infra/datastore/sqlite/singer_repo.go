package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/kawabatas/mini-web-app/internal/domain/model"
)

type SingerRepo struct{ db *sql.DB }

func NewSingerRepo(db *sql.DB) *SingerRepo { return &SingerRepo{db: db} }

func (r *SingerRepo) List(ctx context.Context, offset, limit int) ([]model.Singer, error) {
	if offset < 0 || limit <= 0 {
		return nil, fmt.Errorf("invalid offset/limit: %d/%d", offset, limit)
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, name, genre, debut_year, created_at
FROM singers
ORDER BY id ASC
LIMIT ? OFFSET ?
`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Singer
	for rows.Next() {
		var s model.Singer
		if err := rows.Scan(&s.ID, &s.Name, &s.Genre, &s.DebutYear, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
