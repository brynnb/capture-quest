package main

import (
	"context"
	"database/sql"

	"capturequest/internal/extractorcontract"
)

// Keep the command's importer context name while the source-contract logic is
// shared with other extractor consumers.
type extractorImportContext = extractorcontract.Context

func negotiateExtractorSource(ctx context.Context, sqlite *sql.DB, requestedRelease string) (extractorImportContext, error) {
	return extractorcontract.Negotiate(ctx, sqlite, requestedRelease)
}
