package scriptedevents

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"capturequest/internal/testdb"
)

func TestSyncPublishesAllFamiliesOnlyAfterCommit(t *testing.T) {
	for _, failure := range []string{"late dialogue lookup", "deferred commit constraint", "late cancellation"} {
		t.Run(failure, func(t *testing.T) {
			database := testdb.Postgres(t)
			root := t.TempDir()
			if _, err := database.Exec(`INSERT INTO phaser_dialogue_text (label, source_file, dialogue) VALUES ('ATOMIC_TEXT', 'test', 'hello')`); err != nil {
				t.Fatal(err)
			}
			writeAtomicSyncFiles(t, root, "old", "ATOMIC_TEXT", false)
			if _, err := Sync(context.Background(), database, root); err != nil {
				t.Fatal(err)
			}
			// A failed publication must also roll back its schema upgrades.
			if failure == "late dialogue lookup" {
				if _, err := database.Exec(`ALTER TABLE phaser_cutscene_scripts DROP COLUMN requires_money`); err != nil {
					t.Fatal(err)
				}
			}
			before := snapshotScriptedFamily(t, database)
			dialogue, missingItem := "ATOMIC_TEXT", false
			ctx := context.Background()
			switch failure {
			case "late dialogue lookup":
				dialogue = "MISSING_ATOMIC_TEXT"
			case "deferred commit constraint":
				if _, err := database.Exec(`ALTER TABLE phaser_cutscene_scripts ADD CONSTRAINT test_item_ref FOREIGN KEY (requires_item_id) REFERENCES cq_items(id) DEFERRABLE INITIALLY DEFERRED`); err != nil {
					t.Fatal(err)
				}
				missingItem = true
			case "late cancellation":
				// Sequence advancement survives rollback and proves cancellation reached the
				// visibility insertion, after scripts and coordinates had already changed.
				if _, err := database.Exec(`CREATE SEQUENCE sync_stage;
     CREATE FUNCTION pause_sync() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
      PERFORM nextval('sync_stage'); PERFORM pg_sleep(10); RETURN NEW;
     END $$;
     CREATE TRIGGER pause_sync BEFORE INSERT ON phaser_event_object_visibility FOR EACH ROW EXECUTE FUNCTION pause_sync()`); err != nil {
					t.Fatal(err)
				}
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 200*time.Millisecond)
				defer cancel()
			}
			writeAtomicSyncFiles(t, root, "new", dialogue, missingItem)
			stats, err := Sync(ctx, database, root)
			if err == nil {
				t.Fatal("expected publication failure")
			}
			if stats != (syncStats{}) {
				t.Fatalf("failed publication reported applied stats: %+v", stats)
			}
			if failure == "late cancellation" {
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("cancellation: %v", err)
				}
				var called bool
				if err := database.QueryRow(`SELECT is_called FROM sync_stage`).Scan(&called); err != nil {
					t.Fatal(err)
				}
				if !called {
					t.Fatal("cancellation did not reach late insertion")
				}
				if _, err := database.Exec(`DROP TRIGGER pause_sync ON phaser_event_object_visibility`); err != nil {
					t.Fatal(err)
				}
			}
			if after := snapshotScriptedFamily(t, database); !reflect.DeepEqual(before, after) {
				t.Fatalf("failed publication changed family:\nbefore=%v\nafter=%v", before, after)
			}
			writeAtomicSyncFiles(t, root, "new", "ATOMIC_TEXT", false)
			stats, err = Sync(context.Background(), database, root)
			if err != nil {
				t.Fatalf("retry: %v", err)
			}
			if stats.ScriptsChanged != 1 || !stats.TriggerRowsChanged || !stats.VisibilityChanged || !stats.EventTilesChanged || !stats.ConditionalDialogueChanged {
				t.Fatalf("retry did not publish all families: %+v", stats)
			}
			published := snapshotScriptedFamily(t, database)
			if reflect.DeepEqual(before, published) {
				t.Fatal("retry did not change stored family")
			}
			stats, err = Sync(context.Background(), database, root)
			if err != nil {
				t.Fatal(err)
			}
			if stats.ScriptsChanged != 0 || stats.TriggerRowsChanged || stats.VisibilityChanged || stats.EventTilesChanged || stats.ConditionalDialogueChanged {
				t.Fatalf("duplicate sync rewrote content: %+v", stats)
			}
			if after := snapshotScriptedFamily(t, database); !reflect.DeepEqual(published, after) {
				t.Fatal("duplicate sync changed stored identities")
			}
		})
	}
}

func writeAtomicSyncFiles(t *testing.T, root, version, dialogue string, missingItem bool) {
	t.Helper()
	item := ""
	if missingItem {
		item = `,"requiresItemId":1234567`
	}
	x := 1
	if version == "new" {
		x = 2
	}
	writeScriptedEventFile(t, root, scriptsDirName, "atomic.json", fmt.Sprintf(`{
  "scriptLabel":"AtomicScript","mapName":"TEST","trigger":{"type":"coord","label":"ATOMIC_COORD","coordinates":[{"mapId":1,"mapName":"TEST","x":%d,"y":1}]},
  "actions":[{"type":"dialogue","text":"%s"}]%s}`, x, version, item))
	files := map[string]string{
		visibilityFileName:          fmt.Sprintf(`[{"mapId":1,"mapName":"TEST","objectName":"%s","visible":true}]`, version),
		eventTilesFileName:          fmt.Sprintf(`{"tiles":[{"mapId":1,"mapName":"TEST","x":%d,"y":1,"tileImageId":1}]}`, x),
		conditionalDialogueFileName: fmt.Sprintf(`{"rows":[{"textConstant":"%s","dialogueLabels":["%s"]}]}`, version, dialogue),
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func snapshotScriptedFamily(t *testing.T, database *sql.DB) map[string]string {
	t.Helper()
	result := make(map[string]string)
	for _, table := range []string{"phaser_cutscene_scripts", "phaser_coordinate_triggers", "phaser_event_object_visibility", "phaser_event_tile_overrides", "phaser_conditional_dialogue"} {
		var value string
		if err := database.QueryRow(`SELECT COALESCE(jsonb_agg(to_jsonb(t) ORDER BY id), '[]'::jsonb)::text FROM ` + table + ` t`).Scan(&value); err != nil {
			t.Fatal(err)
		}
		result[table] = value
	}
	return result
}

func TestConcurrentSyncPublishersObserveCommittedFamily(t *testing.T) {
	database := testdb.Postgres(t)
	root := t.TempDir()
	if _, err := database.Exec(`INSERT INTO phaser_dialogue_text (label, source_file, dialogue) VALUES ('ATOMIC_TEXT', 'test', 'hello')`); err != nil {
		t.Fatal(err)
	}
	writeAtomicSyncFiles(t, root, "new", "ATOMIC_TEXT", false)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	type outcome struct {
		stats syncStats
		err   error
	}
	results := make(chan outcome, 4)
	start := make(chan struct{})
	for i := 0; i < 4; i++ {
		go func() {
			<-start
			stats, err := Sync(ctx, database, root)
			results <- outcome{stats, err}
		}()
	}
	close(start)
	changes := 0
	for i := 0; i < 4; i++ {
		result := <-results
		if result.err != nil {
			t.Errorf("concurrent publication: %v", result.err)
			continue
		}
		if result.stats.ScriptsChanged == 1 {
			changes++
			if !result.stats.TriggerRowsChanged || !result.stats.VisibilityChanged || !result.stats.EventTilesChanged || !result.stats.ConditionalDialogueChanged {
				t.Errorf("partial family stats: %+v", result.stats)
			}
		} else if result.stats.ScriptsChanged != 0 || result.stats.TriggerRowsChanged || result.stats.VisibilityChanged || result.stats.EventTilesChanged || result.stats.ConditionalDialogueChanged {
			t.Errorf("publisher rewrote committed family: %+v", result.stats)
		}
	}
	if changes != 1 {
		t.Fatalf("changed publishers = %d, want 1", changes)
	}
	for table := range snapshotScriptedFamily(t, database) {
		var count int
		if err := database.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("%s rows = %d, want 1", table, count)
		}
	}
}
