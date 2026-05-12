package variable

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/s0x401/xdfile-manager/src/internal/utils"

	"github.com/adrg/xdg"
)

const (
	CurrentVersion = "v1.1.0"
	DisplayVersion = "1.1"
	// Allowing pre-releases with non production version
	// Set this to "" for production releases
	PreReleaseSuffix = ""

	RuntimeDirName = "xdfile-data"

	// This will not break in windows. This is a relative path for Embed FS. It uses "/" only
	EmbedConfigDir           = "src/xdfile_config"
	EmbedConfigFile          = EmbedConfigDir + "/config.toml"
	EmbedHotkeysFile         = EmbedConfigDir + "/hotkeys.toml"
	EmbedThemeDir            = EmbedConfigDir + "/theme"
	EmbedThemeCatppuccinFile = EmbedThemeDir + "/catppuccin-mocha.toml"
)

func resolveXdfileBaseDir() string {
	if exePath, err := os.Executable(); err == nil && exePath != "" {
		if resolved, resolveErr := filepath.EvalSymlinks(exePath); resolveErr == nil && resolved != "" {
			exePath = resolved
		}
		exeDir := filepath.Dir(exePath)
		lowerExeDir := strings.ToLower(exeDir)
		lowerTempDir := strings.ToLower(os.TempDir())
		if strings.Contains(lowerExeDir, "go-build") || (lowerTempDir != "" && strings.HasPrefix(lowerExeDir, lowerTempDir)) {
			if wd, wdErr := os.Getwd(); wdErr == nil && wd != "" {
				return wd
			}
		}
		if exeDir != "" {
			return exeDir
		}
	}
	if wd, err := os.Getwd(); err == nil && wd != "" {
		return wd
	}
	return "."
}

var (
	HomeDir        = xdg.Home
	XdfileBaseDir  = resolveXdfileBaseDir()
	XdfileMainDir  = filepath.Join(XdfileBaseDir, RuntimeDirName)
	XdfileCacheDir = filepath.Join(XdfileMainDir, "cache")
	XdfileDataDir  = XdfileMainDir
	XdfileStateDir = XdfileMainDir

	// MainDir files
	ThemeFolder = filepath.Join(XdfileMainDir, "xdfile-theme")

	// DataDir files
	LastCheckVersion = filepath.Join(XdfileDataDir, "xdfile-last-check-version")
	ThemeFileVersion = filepath.Join(XdfileDataDir, "xdfile-theme-version")
	FirstUseCheck    = filepath.Join(XdfileDataDir, "xdfile-first-use")
	PinnedFile       = filepath.Join(XdfileDataDir, "xdfile-pinned.json")
	ToggleDotFile    = filepath.Join(XdfileDataDir, "xdfile-toggle-dot")
	ToggleFooter     = filepath.Join(XdfileDataDir, "xdfile-toggle-footer")

	// StateDir files
	LogFile     = filepath.Join(XdfileStateDir, "xdfile.log")
	LastDirFile = filepath.Join(XdfileStateDir, "xdfile-lastdir")

	// Trash Directories
	DarwinTrashDirectory = filepath.Join(HomeDir, ".Trash")

	// These are used by github.com/rkoesters/xdg/trash package
	// We need to make sure that these directories exist
	LinuxTrashDirectory      = filepath.Join(xdg.DataHome, "Trash")
	LinuxTrashDirectoryFiles = filepath.Join(xdg.DataHome, "Trash", "files")
	LinuxTrashDirectoryInfo  = filepath.Join(xdg.DataHome, "Trash", "info")
)

// These variables are actually not fixed, they are sometimes updated dynamically
var (
	ConfigFile  = filepath.Join(XdfileMainDir, "xdfile-config.toml")
	HotkeysFile = filepath.Join(XdfileMainDir, "xdfile-hotkeys.toml")

	// ChooserFile is the path where Xdfile Manager will write the file's path, which is to be
	// opened, before exiting
	ChooserFile = ""

	// Other state variables
	FixHotkeys    = false
	FixConfigFile = false
	LastDir       = ""
	PrintLastDir  = false
)

// Still we are preventing other packages to directly modify them via reassign linter

func SetLastDir(path string) {
	LastDir = path
}

func SetChooserFile(path string) {
	ChooserFile = path
}

func UpdateVarFromCliArgs(c *cli.Command) {
	// Setting the config file path
	configFileArg := c.String("config-file")

	// Validate the config file exists
	if configFileArg != "" {
		if _, err := os.Stat(configFileArg); err != nil {
			utils.PrintfAndExitf("Error: While reading config file '%s' from argument : %v", configFileArg, err)
		}
		ConfigFile = configFileArg
	}

	hotkeyFileArg := c.String("hotkey-file")

	if hotkeyFileArg != "" {
		if _, err := os.Stat(hotkeyFileArg); err != nil {
			utils.PrintfAndExitf("Error: While reading hotkey file '%s' from argument : %v", hotkeyFileArg, err)
		}
		HotkeysFile = hotkeyFileArg
	}

	// It could be non existent. We are writing to the file. If file doesn't exists, we would attempt to create it.
	SetChooserFile(c.String("chooser-file"))

	FixHotkeys = c.Bool("fix-hotkeys")
	FixConfigFile = c.Bool("fix-config-file")
	PrintLastDir = c.Bool("print-last-dir")
}
