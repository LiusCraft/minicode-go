package config

import (
	"os"
	"path/filepath"
)

const (
	DirName          = ".minioc"
	AssetsDirName    = "assets"
	ConfigFileName   = "minioc.json"
	SessionsDirName  = "sessions"
	ConfigPath       = DirName + "/" + ConfigFileName
	SessionsPath     = DirName + "/" + SessionsDirName
	AssetsConfigPath = AssetsDirName + "/" + ConfigFileName
)

// GlobalConfigDir returns the platform-specific global config directory.
// On Unix-like systems it is ~/.config/minioc.
// On Windows it is %APPDATA%\minioc.
func GlobalConfigDir() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "minioc")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "minioc")
	}
	return filepath.Join(home, ".config", "minioc")
}

func GlobalConfigFile() string {
	return filepath.Join(GlobalConfigDir(), ConfigFileName)
}

func Root(repoRoot string) string {
	return filepath.Join(repoRoot, DirName)
}

func ConfigFile(repoRoot string) string {
	return filepath.Join(Root(repoRoot), ConfigFileName)
}

func AssetsRoot(repoRoot string) string {
	return filepath.Join(repoRoot, AssetsDirName)
}

func AssetsConfigFile(repoRoot string) string {
	return filepath.Join(AssetsRoot(repoRoot), ConfigFileName)
}

func SessionsDir(repoRoot string) string {
	return filepath.Join(Root(repoRoot), SessionsDirName)
}
