package configdir

import (
	"os"

	"golang.org/x/sys/windows"
)

var hasVendorName = true
var systemSettingFolders = []string{knownFolderPath(windows.FOLDERID_ProgramData, "PROGRAMDATA")}
var globalSettingFolder = knownFolderPath(windows.FOLDERID_RoamingAppData, "APPDATA")
var confinedSettingFolder string
var cacheFolder = knownFolderPath(windows.FOLDERID_LocalAppData, "LOCALAPPDATA")

// knownFolderPath asks SHGetKnownFolderPath first because environment
// variables can be missing (e.g. with networked profile folders), and falls
// back to the environment variable when the API fails.
func knownFolderPath(id *windows.KNOWNFOLDERID, env string) string {
	if path, err := windows.KnownFolderPath(id, 0); err == nil && path != "" {
		return path
	}
	return os.Getenv(env)
}
