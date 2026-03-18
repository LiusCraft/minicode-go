package config

import "path/filepath"

const (
	DirName          = ".minioc"
	AssetsDirName    = "assets"
	ConfigFileName   = "minioc.json"
	SessionsDirName  = "sessions"
	ConfigPath       = DirName + "/" + ConfigFileName
	SessionsPath     = DirName + "/" + SessionsDirName
	AssetsConfigPath = AssetsDirName + "/" + ConfigFileName
)

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
