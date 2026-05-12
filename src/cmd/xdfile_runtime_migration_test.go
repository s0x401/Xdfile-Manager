package cmd

import (
	"os"
	"path/filepath"
	"testing"

	variable "github.com/s0x401/xdfile-manager/src/config"
)

func TestMigrateLegacyRuntimeFilesMovesRootFilesIntoDataDir(t *testing.T) {
	restore := captureXdfileRuntimeDirs()
	defer restore()

	baseDir := t.TempDir()
	configureXdfileRuntimeDirsForTest(baseDir)

	legacyConfig := filepath.Join(baseDir, "xdfile-config.toml")
	legacyLayout := filepath.Join(baseDir, xdfileLayoutFileName)
	legacyThemeDir := filepath.Join(baseDir, "xdfile-theme")
	if err := os.WriteFile(legacyConfig, []byte("config"), 0o600); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}
	if err := os.WriteFile(legacyLayout, []byte("{}"), 0o600); err != nil {
		t.Fatalf("write legacy layout: %v", err)
	}
	if err := os.MkdirAll(legacyThemeDir, 0o700); err != nil {
		t.Fatalf("create legacy theme dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyThemeDir, "custom.toml"), []byte("theme"), 0o600); err != nil {
		t.Fatalf("write legacy theme: %v", err)
	}

	migrateLegacyRuntimeFiles()

	if _, err := os.Stat(legacyConfig); !os.IsNotExist(err) {
		t.Fatalf("expected legacy config to move away, stat err=%v", err)
	}
	if data, err := os.ReadFile(variable.ConfigFile); err != nil || string(data) != "config" {
		t.Fatalf("expected config to be migrated, data=%q err=%v", data, err)
	}
	if _, err := os.Stat(legacyLayout); !os.IsNotExist(err) {
		t.Fatalf("expected legacy layout to move away, stat err=%v", err)
	}
	if _, err := os.Stat(xdfileLayoutPrefsPath()); err != nil {
		t.Fatalf("expected layout to be migrated: %v", err)
	}
	if _, err := os.Stat(filepath.Join(variable.ThemeFolder, "custom.toml")); err != nil {
		t.Fatalf("expected theme folder to be migrated: %v", err)
	}
}

func TestMigrateLegacyRuntimeFilesPreservesExistingTargets(t *testing.T) {
	restore := captureXdfileRuntimeDirs()
	defer restore()

	baseDir := t.TempDir()
	configureXdfileRuntimeDirsForTest(baseDir)
	if err := os.MkdirAll(variable.XdfileMainDir, 0o700); err != nil {
		t.Fatalf("create runtime dir: %v", err)
	}
	if err := os.WriteFile(variable.ConfigFile, []byte("new"), 0o600); err != nil {
		t.Fatalf("write current config: %v", err)
	}
	legacyConfig := filepath.Join(baseDir, "xdfile-config.toml")
	if err := os.WriteFile(legacyConfig, []byte("old"), 0o600); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	migrateLegacyRuntimeFiles()

	if data, err := os.ReadFile(variable.ConfigFile); err != nil || string(data) != "new" {
		t.Fatalf("expected current config to remain untouched, data=%q err=%v", data, err)
	}
	legacyCopy := filepath.Join(variable.XdfileMainDir, "legacy-root", "xdfile-config.toml")
	if data, err := os.ReadFile(legacyCopy); err != nil || string(data) != "old" {
		t.Fatalf("expected legacy config copy, data=%q err=%v", data, err)
	}
	if _, err := os.Stat(legacyConfig); !os.IsNotExist(err) {
		t.Fatalf("expected legacy root config to move away, stat err=%v", err)
	}
}

func captureXdfileRuntimeDirs() func() {
	homeDir := variable.HomeDir
	baseDir := variable.XdfileBaseDir
	mainDir := variable.XdfileMainDir
	cacheDir := variable.XdfileCacheDir
	dataDir := variable.XdfileDataDir
	stateDir := variable.XdfileStateDir
	themeFolder := variable.ThemeFolder
	lastCheckVersion := variable.LastCheckVersion
	themeFileVersion := variable.ThemeFileVersion
	firstUseCheck := variable.FirstUseCheck
	pinnedFile := variable.PinnedFile
	toggleDotFile := variable.ToggleDotFile
	toggleFooter := variable.ToggleFooter
	logFile := variable.LogFile
	lastDirFile := variable.LastDirFile
	configFile := variable.ConfigFile
	hotkeysFile := variable.HotkeysFile
	return func() {
		variable.HomeDir = homeDir
		variable.XdfileBaseDir = baseDir
		variable.XdfileMainDir = mainDir
		variable.XdfileCacheDir = cacheDir
		variable.XdfileDataDir = dataDir
		variable.XdfileStateDir = stateDir
		variable.ThemeFolder = themeFolder
		variable.LastCheckVersion = lastCheckVersion
		variable.ThemeFileVersion = themeFileVersion
		variable.FirstUseCheck = firstUseCheck
		variable.PinnedFile = pinnedFile
		variable.ToggleDotFile = toggleDotFile
		variable.ToggleFooter = toggleFooter
		variable.LogFile = logFile
		variable.LastDirFile = lastDirFile
		variable.ConfigFile = configFile
		variable.HotkeysFile = hotkeysFile
	}
}

func configureXdfileRuntimeDirsForTest(baseDir string) {
	variable.XdfileBaseDir = baseDir
	variable.XdfileMainDir = filepath.Join(baseDir, variable.RuntimeDirName)
	variable.XdfileCacheDir = filepath.Join(variable.XdfileMainDir, "cache")
	variable.XdfileDataDir = variable.XdfileMainDir
	variable.XdfileStateDir = variable.XdfileMainDir
	variable.ThemeFolder = filepath.Join(variable.XdfileMainDir, "xdfile-theme")
	variable.LastCheckVersion = filepath.Join(variable.XdfileDataDir, "xdfile-last-check-version")
	variable.ThemeFileVersion = filepath.Join(variable.XdfileDataDir, "xdfile-theme-version")
	variable.FirstUseCheck = filepath.Join(variable.XdfileDataDir, "xdfile-first-use")
	variable.PinnedFile = filepath.Join(variable.XdfileDataDir, "xdfile-pinned.json")
	variable.ToggleDotFile = filepath.Join(variable.XdfileDataDir, "xdfile-toggle-dot")
	variable.ToggleFooter = filepath.Join(variable.XdfileDataDir, "xdfile-toggle-footer")
	variable.LogFile = filepath.Join(variable.XdfileStateDir, "xdfile.log")
	variable.LastDirFile = filepath.Join(variable.XdfileStateDir, "xdfile-lastdir")
	variable.ConfigFile = filepath.Join(variable.XdfileMainDir, "xdfile-config.toml")
	variable.HotkeysFile = filepath.Join(variable.XdfileMainDir, "xdfile-hotkeys.toml")
}
