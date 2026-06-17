package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"

	"capturequest/internal/scriptsim"
)

type runOptions struct {
	check   bool
	update  bool
	verbose bool
}

func main() {
	scenarioName := flag.String("scenario", "", "scenario name or JSON path")
	all := flag.Bool("all", false, "run every scenario in script_tests/scenarios")
	check := flag.Bool("check", false, "compare output to golden file")
	update := flag.Bool("update", false, "write output to golden file")
	verbose := flag.Bool("verbose", false, "print output even when --check passes")
	flag.Parse()

	if *all && *scenarioName != "" {
		log.Fatal("use either --scenario or --all, not both")
	}
	if !*all && *scenarioName == "" {
		log.Fatal("--scenario or --all is required")
	}
	if err := scriptsim.InitDB(); err != nil {
		log.Fatalf("database init failed: %v", err)
	}

	opts := runOptions{check: *check, update: *update, verbose: *verbose}
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
			if err := runScenario(path, opts); err != nil {
				log.Fatal(err)
			}
		}
		return
	}

	if err := runScenario(scriptsim.ScenarioPath(*scenarioName), opts); err != nil {
		log.Fatal(err)
	}
}

func runScenario(scenarioPath string, opts runOptions) error {
	scenario, err := scriptsim.LoadScenario(scenarioPath)
	if err != nil {
		return fmt.Errorf("load scenario failed: %w", err)
	}
	result, err := scriptsim.Run(scenario)
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
