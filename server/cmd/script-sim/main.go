package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"syscall"

	"capturequest/internal/db"
	"capturequest/internal/scriptcandidateimport"
	"capturequest/internal/scriptsim"
	"database/sql"
)

type runOptions struct {
	sourcePath string
	resolver   **scriptcandidateimport.TileIdentityResolver
	check      bool
	update     bool
	verbose    bool
}

func main() {
	scenarioName := flag.String("scenario", "", "scenario name or JSON path")
	all := flag.Bool("all", false, "run every scenario in script_tests/scenarios")
	check := flag.Bool("check", false, "compare output to golden file")
	update := flag.Bool("update", false, "write output to golden file")
	verbose := flag.Bool("verbose", false, "print output even when --check passes")
	sourcePath := flag.String("tile-source", "../public/phaser/pokemon.db", "negotiated SQLite source for native tile expectations")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *all && *scenarioName != "" {
		log.Fatal("use either --scenario or --all, not both")
	}
	if !*all && *scenarioName == "" {
		log.Fatal("--scenario or --all is required")
	}
	if err := scriptsim.InitDB(ctx); err != nil {
		log.Fatalf("database init failed: %v", err)
	}

	var resolver *scriptcandidateimport.TileIdentityResolver
	opts := runOptions{check: *check, update: *update, verbose: *verbose, sourcePath: *sourcePath, resolver: &resolver}
	if *all {
		paths, err := filepath.Glob(filepath.Join("script_tests", "scenarios", "*.json"))
		if err != nil {
			log.Fatalf("find scenarios failed: %v", err)
		}
		if len(paths) == 0 {
			log.Fatal("no scenarios found")
		}
		sort.Strings(paths)
		for _, path := range paths {
			if err := runScenario(ctx, path, opts); err != nil {
				log.Fatal(err)
			}
		}
		return
	}

	if err := runScenario(ctx, scriptsim.ScenarioPath(*scenarioName), opts); err != nil {
		log.Fatal(err)
	}
}

func runScenario(ctx context.Context, scenarioPath string, opts runOptions) error {
	scenario, err := scriptsim.LoadScenario(scenarioPath)
	if err != nil {
		return fmt.Errorf("load scenario failed: %w", err)
	}
	for _, expected := range scenario.Expect.TileStates {
		if expected.Source == nil {
			continue
		}
		if *opts.resolver == nil {
			var release string
			if err := db.GlobalWorldDB.DB.QueryRowContext(ctx, `SELECT release_code FROM phaser_import_metadata WHERE singleton=true`).Scan(&release); err != nil {
				return err
			}
			absolute, err := filepath.Abs(opts.sourcePath)
			if err != nil {
				return err
			}
			uri := url.URL{Scheme: "file", Path: absolute, RawQuery: "mode=ro"}
			source, err := sql.Open("sqlite", uri.String())
			if err != nil {
				return err
			}
			source.SetMaxOpenConns(1)
			*opts.resolver, err = scriptcandidateimport.NewTileIdentityResolver(ctx, source, release)
			source.Close()
			if err != nil {
				return err
			}
		}
		if err := scriptsim.ResolveTileExpectations(ctx, db.GlobalWorldDB.DB, *opts.resolver, scenario); err != nil {
			return err
		}
		break
	}
	result, err := scriptsim.Run(ctx, db.GlobalWorldDB.DB, scenario)
	output := ""
	if result != nil {
		output = scriptsim.FormatResult(result)
	}
	if err != nil {
		if output != "" {
			fmt.Print(output)
		}
		return fmt.Errorf("scenario failed: %w", err)
	}

	goldenPath := scriptsim.GoldenPath(scenario.Name)
	if opts.update {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			return fmt.Errorf("create golden dir failed: %w", err)
		}
		if err := os.WriteFile(goldenPath, []byte(output), 0o644); err != nil {
			return fmt.Errorf("write golden failed: %w", err)
		}
		fmt.Printf("updated %s\n", goldenPath)
	}
	if opts.check {
		expected, err := os.ReadFile(goldenPath)
		if err != nil {
			return fmt.Errorf("read golden failed: %w", err)
		}
		if !bytes.Equal(bytes.TrimSpace(expected), bytes.TrimSpace([]byte(output))) {
			fmt.Print(output)
			return fmt.Errorf("golden mismatch: %s", goldenPath)
		}
		if opts.verbose {
			fmt.Print(output)
		}
		fmt.Printf("PASS %s\n", scenario.Name)
		return nil
	}
	if !opts.update || opts.verbose {
		fmt.Print(output)
	}
	return nil
}
