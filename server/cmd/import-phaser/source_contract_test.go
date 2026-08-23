package main

import (
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func extractorContractFixture(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	for _, table := range requiredExtractorTables {
		var definition string
		switch table {
		case "schema_metadata":
			definition = `(schema_name TEXT, schema_version INTEGER, minimum_reader_version INTEGER, applied_epoch INTEGER)`
		case "extraction_runs":
			definition = `(run_id TEXT, schema_name TEXT, schema_version INTEGER, extractor_revision TEXT, source_revision TEXT, source_date_epoch INTEGER, source_root TEXT, source_tree_sha256 TEXT)`
		case "game_releases":
			definition = `(release_code TEXT, title TEXT, variant TEXT, platform TEXT, region TEXT, language TEXT, build_define TEXT)`
		case "extraction_run_releases":
			definition = `(run_id TEXT, release_code TEXT)`
		case "pokemon":
			definition = `(id INTEGER, default_move_1_id INTEGER, default_move_1_name TEXT, default_move_2_id INTEGER, default_move_2_name TEXT, default_move_3_id INTEGER, default_move_3_name TEXT, default_move_4_id INTEGER, default_move_4_name TEXT)`
		case "moves":
			definition = `(id INTEGER, constant_name TEXT)`
		default:
			definition = `(id INTEGER)`
		}
		if _, err := db.Exec(`CREATE TABLE ` + table + ` ` + definition); err != nil {
			t.Fatalf("create %s: %v", table, err)
		}
	}
	statements := []string{
		`INSERT INTO schema_metadata VALUES ('pokemon-gameboy-extractor', 2, 2, 1700000000)`,
		`INSERT INTO extraction_runs VALUES ('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'pokemon-gameboy-extractor', 2, 'extractor-revision', 'source-revision', 1700000000, 'pokemon-game-data', 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb')`,
		`INSERT INTO game_releases VALUES ('red', 'Pokemon Red', 'red', 'game-boy', 'international', 'en', '_RED')`,
		`INSERT INTO game_releases VALUES ('blue', 'Pokemon Blue', 'blue', 'game-boy', 'international', 'en', '_BLUE')`,
		`INSERT INTO extraction_run_releases VALUES ('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'red')`,
		`INSERT INTO extraction_run_releases VALUES ('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'blue')`,
		`INSERT INTO moves VALUES (28, 'SAND_ATTACK')`,
		`INSERT INTO moves VALUES (94, 'PSYCHIC_M')`,
		`INSERT INTO pokemon VALUES (1, 28, 'SAND_ATTACK', 94, 'PSYCHIC_M', NULL, 'NO_MOVE', NULL, 'NO_MOVE')`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestNegotiateExtractorSourceAcceptsV2AndMoveConstants(t *testing.T) {
	context, err := negotiateExtractorSource(extractorContractFixture(t), "BLUE")
	if err != nil {
		t.Fatal(err)
	}
	if context.ReleaseCode != "blue" || context.SchemaVersion != 2 {
		t.Fatalf("unexpected context: %+v", context)
	}
}

func TestNegotiateExtractorSourceRejectsBeforeImport(t *testing.T) {
	db := extractorContractFixture(t)
	if _, err := db.Exec(`UPDATE schema_metadata SET schema_version = 3`); err != nil {
		t.Fatal(err)
	}
	_, err := negotiateExtractorSource(db, "red")
	if err == nil || !strings.Contains(err.Error(), "unsupported extractor schema version 3") {
		t.Fatalf("error = %v", err)
	}
}

func TestNegotiateExtractorSourceRejectsMismatchedDefaultConstant(t *testing.T) {
	db := extractorContractFixture(t)
	if _, err := db.Exec(`UPDATE pokemon SET default_move_1_name = 'SAND-ATTACK'`); err != nil {
		t.Fatal(err)
	}
	_, err := negotiateExtractorSource(db, "red")
	if err == nil || !strings.Contains(err.Error(), `moves.constant_name="SAND_ATTACK"`) {
		t.Fatalf("error = %v", err)
	}
}

func TestNegotiateExtractorSourceRequiresCompleteTables(t *testing.T) {
	db := extractorContractFixture(t)
	if _, err := db.Exec(`DROP TABLE warps`); err != nil {
		t.Fatal(err)
	}
	_, err := negotiateExtractorSource(db, "red")
	if err == nil || !strings.Contains(err.Error(), "missing required tables: warps") {
		t.Fatalf("error = %v", err)
	}
}
