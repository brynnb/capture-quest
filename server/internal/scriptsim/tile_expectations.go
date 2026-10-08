package scriptsim

import (
	"capturequest/internal/scriptcandidateimport"
	"context"
	"database/sql"
	"fmt"
)

// Resolve expectations from native blocks, not runtime rule outputs or old IDs.
func ResolveTileExpectations(ctx context.Context, database *sql.DB, resolver *scriptcandidateimport.TileIdentityResolver, scenario *Scenario) error {
	var run, revision, tree, release string
	if err := database.QueryRowContext(ctx, `SELECT extraction_run_id,source_revision,source_tree_sha256,release_code FROM phaser_import_metadata WHERE singleton=true`).Scan(&run, &revision, &tree, &release); err != nil {
		return err
	}
	contract := resolver.Contract
	if run != contract.RunID || revision != contract.SourceRevision || tree != contract.SourceTreeSHA256 || release != contract.ReleaseCode {
		return fmt.Errorf("tile expectation source differs from imported extractor contract")
	}
	resolved := map[int]int{}
	for i := range scenario.Expect.TileStates {
		expected := &scenario.Expect.TileStates[i]
		if expected.Source == nil {
			continue
		}
		if expected.TileImageID != 0 {
			return fmt.Errorf("tile expectation cannot mix numeric and native identity")
		}
		if expected.Source.MapName == "" || expected.Source.BlockID == nil || expected.Source.Position == nil {
			return fmt.Errorf("native tile identity requires mapName, blockId and position")
		}
		id, err := resolver.Resolve(expected.Source.MapName, *expected.Source.BlockID, *expected.Source.Position)
		if err != nil {
			return err
		}
		var tileset, block, position int
		if err := database.QueryRowContext(ctx, `SELECT tileset_id,block_index,position FROM phaser_tile_images WHERE id=$1`, id).Scan(&tileset, &block, &position); err != nil {
			return err
		}
		if err := resolver.VerifyCatalogIdentity(id, tileset, block, position); err != nil {
			return err
		}
		resolved[i] = id
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	for i, id := range resolved {
		scenario.Expect.TileStates[i].TileImageID = id
	}
	return nil
}
