//go:build !windows && !darwin

package configdir

import (
	"os"
	"path/filepath"
)

var hasVendorName = true
var systemSettingFolders []string
var globalSettingFolder string
var confinedSettingFolder string
var cacheFolder string

func init() {
	systemSettingFolders, globalSettingFolder, cacheFolder = xdgFolders(os.Getenv("HOME"))
	// Inside snap, $HOME points to the snap private folder ($HOME/snap/<name>/<revision>).
	// The real home folder (accessible with personal-files interface) is preferred,
	// and the snap private folder is used as a confined folder.
	if realHome := os.Getenv("SNAP_REAL_HOME"); realHome != "" && realHome != os.Getenv("HOME") {
		confinedSettingFolder = globalSettingFolder
		globalSettingFolder = filepath.Join(realHome, ".config")
	}
}
