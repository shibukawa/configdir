package configdir

import (
	"os"
	"path/filepath"
	"strings"
)

// https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html

// xdgFolders returns folders defined by XDG Base Directory Specification.
func xdgFolders(home string) (system []string, global, cache string) {
	if os.Getenv("XDG_CONFIG_HOME") != "" {
		global = os.Getenv("XDG_CONFIG_HOME")
	} else {
		global = filepath.Join(home, ".config")
	}
	if os.Getenv("XDG_CONFIG_DIRS") != "" {
		system = strings.Split(os.Getenv("XDG_CONFIG_DIRS"), ":")
	} else {
		system = []string{"/etc/xdg"}
	}
	if os.Getenv("XDG_CACHE_HOME") != "" {
		cache = os.Getenv("XDG_CACHE_HOME")
	} else {
		cache = filepath.Join(home, ".cache")
	}
	return
}
