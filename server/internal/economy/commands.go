package economy

import (
	"capturequest/internal/db/cqitems"
	"context"
)

func (s *Service) ValidateSchema(ctx context.Context) error {
	return cqitems.NewStore(s.database).ValidateCommandSchema(ctx)
}
