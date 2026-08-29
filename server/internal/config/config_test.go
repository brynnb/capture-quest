package config

import "testing"

func TestServerDefaultsToProductionSafeMode(t *testing.T) {
	previousConfig := config
	previousConfigFile := configFileData
	previousKeyFile := keyPEMData
	t.Cleanup(func() {
		config = previousConfig
		configFileData = previousConfigFile
		keyPEMData = previousKeyFile
	})

	config = nil
	configFileData = t.TempDir() + "/missing-config.json"
	keyPEMData = t.TempDir() + "/missing-key.pem"
	t.Setenv("LOCAL", "")

	cfg, err := Get()
	if err != nil {
		t.Fatalf("load defaults: %v", err)
	}
	if cfg.Local {
		t.Fatal("server enabled local/debug routes without explicit LOCAL=true")
	}
}

func TestServerEnablesLocalModeOnlyWhenExplicit(t *testing.T) {
	previousConfig := config
	previousConfigFile := configFileData
	t.Cleanup(func() {
		config = previousConfig
		configFileData = previousConfigFile
	})

	config = nil
	configFileData = t.TempDir() + "/missing-config.json"
	t.Setenv("LOCAL", "true")

	cfg, err := Get()
	if err != nil {
		t.Fatalf("load explicit local config: %v", err)
	}
	if !cfg.Local {
		t.Fatal("LOCAL=true did not enable local development mode")
	}
}
