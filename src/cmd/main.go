package cmd

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/s0x401/xdfile-manager/src/internal/common"
	"github.com/s0x401/xdfile-manager/src/internal/utils"

	"github.com/charmbracelet/lipgloss"
	"github.com/urfave/cli/v3"

	variable "github.com/s0x401/xdfile-manager/src/config"
)

// Run Xdfile Manager app
func Run(content embed.FS) {
	if xdfileMaybeRunPTYMouseProxy() {
		return
	}

	// Enable custom colored help output
	cli.HelpPrinter = CustomHelpPrinter //nolint:reassign // Intentionally reassigning to customize help output

	// Before we open log file, set all "non debug" logs to stdout
	utils.SetRootLoggerToStdout(false)

	common.LoadInitialPrerenderedVariables()
	common.LoadAllDefaultConfig(content)

	app := &cli.Command{
		Name:        "xdfile",
		Version:     variable.DisplayVersion + variable.PreReleaseSuffix,
		Description: "Xdfile Manager terminal file manager",
		ArgsUsage:   "[PATH]...",
		Commands: []*cli.Command{
			{
				Name:    "path-list",
				Aliases: []string{"pl"},
				Usage:   "Print the path to the Xdfile Manager configuration and data files",
				Action: func(_ context.Context, c *cli.Command) error {
					if c.Bool("lastdir-file") {
						fmt.Println(variable.LastDirFile)
						return nil
					}
					fmt.Printf("%-*s %s\n",
						common.HelpKeyColumnWidth,
						lipgloss.NewStyle().Foreground(lipgloss.Color("#66b2ff")).Render("[Configuration file path]"),
						variable.ConfigFile,
					)
					fmt.Printf("%-*s %s\n",
						common.HelpKeyColumnWidth,
						lipgloss.NewStyle().Foreground(lipgloss.Color("#ffcc66")).Render("[Hotkeys file path]"),
						variable.HotkeysFile,
					)
					fmt.Printf("%-*s %s\n",
						common.HelpKeyColumnWidth,
						lipgloss.NewStyle().Foreground(lipgloss.Color("#66ccff")).Render("[Setup file path]"),
						xdfileLayoutPrefsPath(),
					)
					fmt.Printf("%-*s %s\n",
						common.HelpKeyColumnWidth,
						lipgloss.NewStyle().Foreground(lipgloss.Color("#ff9966")).Render("[User menu file path]"),
						xdfileCommandsPrefsPath(),
					)
					logStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#66ff66"))
					configStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#ff9999"))
					dataStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#ff66ff"))
					fmt.Printf("%-*s %s\n", common.HelpKeyColumnWidth,
						logStyle.Render("[Log file path]"), variable.LogFile)
					fmt.Printf("%-*s %s\n", common.HelpKeyColumnWidth,
						configStyle.Render("[Xdfile Manager directory path]"), variable.XdfileMainDir)
					fmt.Printf("%-*s %s\n", common.HelpKeyColumnWidth,
						dataStyle.Render("[Theme directory path]"), variable.ThemeFolder)
					return nil
				},
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "lastdir-file",
						Aliases: []string{"ld"},
						Usage:   "Print path to lastdir file (Where last dir is written when cd_on_quit config is true)",
						Value:   false,
					},
				},
			},
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "fix-hotkeys",
				Aliases: []string{"fh"},
				Usage:   "Adds any missing hotkeys to the hotkey config file",
				Value:   false,
			},
			&cli.BoolFlag{
				Name:    "fix-config-file",
				Aliases: []string{"fch"},
				Usage:   "Adds any missing hotkeys to the hotkey config file",
				Value:   false,
			},
			&cli.BoolFlag{
				Name:    "print-last-dir",
				Aliases: []string{"pld"},
				Usage:   "Print the last dir to stdout on exit (to use for cd)",
				Value:   false,
			},
			&cli.StringFlag{
				Name:    "config-file",
				Aliases: []string{"c"},
				Usage:   "Specify the path to a different config file",
				Value:   "", // Default to the blank string indicating non-usage of flag
			},
			&cli.StringFlag{
				Name:    "hotkey-file",
				Aliases: []string{"hf"},
				Usage:   "Specify the path to a different hotkey file",
				Value:   "", // Default to the blank string indicating non-usage of flag
			},
			&cli.StringFlag{
				Name:    "chooser-file",
				Aliases: []string{"cf"},
				Usage:   "On trying to open any file, Xdfile Manager will write its path to this file and exit",
				Value:   "", // Default to the blank string indicating non-usage of flag
			},
		},
		Action: xdfileRootAction,
	}

	err := app.Run(context.Background(), os.Args)
	if err != nil {
		utils.PrintlnAndExit(err)
	}
}

func xdfileRootAction(_ context.Context, c *cli.Command) error {
	migrateLegacyRuntimeFiles()
	variable.UpdateVarFromCliArgs(c)
	InitConfigFile()
	if err := utils.SetRootLoggerToFile(variable.LogFile, false); err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	if err := runXdfileApp(c.Args().Slice()); err != nil {
		return err
	}
	return nil
}

type xdfileLegacyRuntimePath struct {
	Legacy string
	Target string
}

func migrateLegacyRuntimeFiles() {
	baseDir := variable.XdfileBaseDir
	legacyPaths := []xdfileLegacyRuntimePath{
		{Legacy: filepath.Join(baseDir, "xdfile-config.toml"), Target: variable.ConfigFile},
		{Legacy: filepath.Join(baseDir, "xdfile-hotkeys.toml"), Target: variable.HotkeysFile},
		{Legacy: filepath.Join(baseDir, xdfileLayoutFileName), Target: xdfileLayoutPrefsPath()},
		{Legacy: filepath.Join(baseDir, xdfileCommandsFileName), Target: xdfileCommandsPrefsPath()},
		{Legacy: filepath.Join(baseDir, xdfileNetBoxFileName), Target: xdfileNetBoxPrefsPath()},
		{Legacy: filepath.Join(baseDir, xdfileDeleteUndoStateFile), Target: xdfileDeleteUndoStatePath()},
		{Legacy: filepath.Join(baseDir, "xdfile.log"), Target: variable.LogFile},
		{Legacy: filepath.Join(baseDir, "xdfile-lastdir"), Target: variable.LastDirFile},
		{Legacy: filepath.Join(baseDir, "xdfile-last-check-version"), Target: variable.LastCheckVersion},
		{Legacy: filepath.Join(baseDir, "xdfile-theme-version"), Target: variable.ThemeFileVersion},
		{Legacy: filepath.Join(baseDir, "xdfile-first-use"), Target: variable.FirstUseCheck},
		{Legacy: filepath.Join(baseDir, "xdfile-pinned.json"), Target: variable.PinnedFile},
		{Legacy: filepath.Join(baseDir, "xdfile-toggle-dot"), Target: variable.ToggleDotFile},
		{Legacy: filepath.Join(baseDir, "xdfile-toggle-footer"), Target: variable.ToggleFooter},
		{Legacy: filepath.Join(baseDir, "xdfile-theme"), Target: variable.ThemeFolder},
	}
	for _, item := range legacyPaths {
		migrateLegacyRuntimePath(item)
	}
}

func migrateLegacyRuntimePath(item xdfileLegacyRuntimePath) {
	legacy := filepath.Clean(item.Legacy)
	target := filepath.Clean(item.Target)
	if legacy == target || legacy == "." || target == "." {
		return
	}
	if _, err := os.Stat(legacy); err != nil {
		return
	}
	if _, err := os.Stat(target); err == nil {
		target = xdfileUniqueMigrationTarget(filepath.Join(variable.XdfileMainDir, "legacy-root", filepath.Base(legacy)))
	}
	if err := os.MkdirAll(filepath.Dir(target), utils.ConfigDirPerm); err != nil {
		slog.Warn("failed to create legacy runtime migration directory", "path", target, "error", err)
		return
	}
	if err := os.Rename(legacy, target); err != nil {
		slog.Warn("failed to migrate legacy runtime file", "from", legacy, "to", target, "error", err)
	}
}

func xdfileUniqueMigrationTarget(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for i := 2; ; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s-%d%s", stem, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

// Create proper directories for storing configuration and write default
// configurations to Config and Hotkeys toml
func InitConfigFile() {
	// Create directories
	if err := utils.CreateDirectories(
		variable.XdfileMainDir,
		variable.XdfileCacheDir,
		variable.XdfileDataDir,
		variable.XdfileStateDir,
		variable.ThemeFolder,
	); err != nil {
		utils.PrintlnAndExit("Error creating directories:", err)
	}

	// Create files
	if err := utils.CreateFiles(
		variable.ToggleDotFile,
		variable.LogFile,
		variable.ThemeFileVersion,
		variable.ToggleFooter,
	); err != nil {
		utils.PrintlnAndExit("Error creating files:", err)
	}

	// Write config file
	if err := writeConfigFile(variable.ConfigFile, common.ConfigTomlString); err != nil {
		utils.PrintlnAndExit("Error writing config file:", err)
	}

	if err := writeConfigFile(variable.HotkeysFile, common.HotkeysTomlString); err != nil {
		utils.PrintlnAndExit("Error writing config file:", err)
	}
}

// Write data to the path file if it does not exists
func writeConfigFile(path, data string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err = os.WriteFile(path, []byte(data), utils.ConfigFilePerm); err != nil {
			return fmt.Errorf("failed to write config file %s: %w", path, err)
		}
	}
	return nil
}
