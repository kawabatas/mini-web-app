package repository

import (
	"context"

	"github.com/kawabatas/mini-web-app/internal/domain/model"
)

// SingerRepository abstracts Singer persistence regardless of the underlying DB.
type SingerRepository interface {
	List(ctx context.Context, offset, limit int) ([]model.Singer, error)
}
