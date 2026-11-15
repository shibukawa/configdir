package configdir

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

// executable is replaced in tests.
var executable = os.Executable

// QueryResourceFolders returns existing folders that contain files installed with the application
// (e.g. default settings, templates and assets) in priority order:
//
//  1. ResourcePath (if set)
//  2. The folder relative to the executable (symbolic links are resolved)
//  3. ${XDG_DATA_DIRS}/<ApplicationName> (/usr/local/share and /usr/share by default)
//     on Linux/BSDs, and on macOS when XDG is true
//
// The folder relative to the executable is:
//
//   - macOS app bundle: X.app/Contents/MacOS/app → X.app/Contents/Resources
//   - Windows: the folder of app.exe
//   - Others: <prefix>/bin/app → <prefix>/share/<ApplicationName>
//     (/usr, /usr/local, Homebrew, snap, Flatpak, AppImage and so on)
//
// Unlike other folders, VendorName is not used because packages are installed to
// <prefix>/share/<package name> by convention.
// The folders are read-only. Don't write files to them.
func (c ConfigDir) QueryResourceFolders() []*Config {
	var candidates []string
	if c.ResourcePath != "" {
		candidates = append(candidates, c.ResourcePath)
	}
	if exe, err := executable(); err == nil {
		if exe, err := filepath.EvalSymlinks(exe); err == nil {
			if folder := exeResourceFolder(runtime.GOOS, exe, c.ApplicationName); folder != "" {
				candidates = append(candidates, folder)
			}
		}
	}
	if runtime.GOOS != "windows" && (runtime.GOOS != "darwin" || c.XDG) {
		for _, root := range xdgDataDirs() {
			candidates = append(candidates, filepath.Join(root, c.ApplicationName))
		}
	}
	var result []*Config
	seen := map[string]bool{}
	for _, path := range candidates {
		path = filepath.Clean(path)
		if seen[path] {
			continue
		}
		seen[path] = true
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			result = append(result, c.newConfig(path, Resource))
		}
	}
	return result
}

// exeResourceFolder returns the resource folder relative to the executable.
func exeResourceFolder(goos, exe, applicationName string) string {
	dir := filepath.Dir(exe)
	if goos == "windows" {
		return dir
	}
	if goos == "darwin" && filepath.Base(dir) == "MacOS" && filepath.Base(filepath.Dir(dir)) == "Contents" {
		return filepath.Join(filepath.Dir(dir), "Resources")
	}
	if filepath.Base(dir) == "bin" {
		return filepath.Join(filepath.Dir(dir), "share", applicationName)
	}
	return ""
}

// ResourceFS returns a read-only fs.FS that resolves each name in the order of
// QueryResourceFolders, and then fallbacks (e.g. embed.FS bundled in the executable).
func (c ConfigDir) ResourceFS(fallbacks ...fs.FS) fs.FS {
	var layers layeredFS
	for _, config := range c.QueryResourceFolders() {
		layers = append(layers, config.FS())
	}
	return append(layers, fallbacks...)
}
