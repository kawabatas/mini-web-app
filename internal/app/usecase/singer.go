package usecase

import (
	"context"

	"github.com/kawabatas/mini-web-app/internal/domain/model"
	"github.com/kawabatas/mini-web-app/internal/infra/datastore"
)

// SingerListParams は /api/singers のリクエストのパラメータです。
type SingerListParams struct {
	Limit  int
	Offset int
}

type SingerListResult struct {
	Items      []model.Singer `json:"items"`
	Total      int            `json:"total"`
	NextOffset int            `json:"next_offset"`
}

type SingerService struct {
	ds datastore.DataStore
}

func NewSingerService(ds datastore.DataStore) *SingerService {
	return &SingerService{ds: ds}
}

func (s *SingerService) List(ctx context.Context, p SingerListParams) (SingerListResult, error) {
	offset, limit := 0, 20
	if p.Offset > 0 {
		offset = p.Offset
	}
	// limit は 1〜100 の範囲に制限
	if limit > 0 && limit <= 100 {
		limit = p.Limit
	}

	base, err := s.ds.Singers().List(ctx, offset, limit+1) // 次ページ確認のため +1 で取得
	if err != nil {
		return SingerListResult{}, err
	}

	next := offset + limit
	if len(base)-1 < limit {
		next = -1 // 次ページなし
	}
	return SingerListResult{Items: base, Total: len(base), NextOffset: next}, nil
}
