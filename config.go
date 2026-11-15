// configdir provides access to configuration folder in each platforms.
//
// System wide configuration folders:
//
//   - Windows: FOLDERID_ProgramData (C:\ProgramData)
//   - Linux/BSDs: ${XDG_CONFIG_DIRS} (/etc/xdg)
//   - MacOSX: "/Library/Application Support"
//
// User wide configuration folders:
//
//   - Windows: FOLDERID_RoamingAppData (C:\Users\<User>\AppData\Roaming)
//   - Linux/BSDs: ${XDG_CONFIG_HOME} (${HOME}/.config)
//   - MacOSX: "${HOME}/Library/Application Support"
//
// User wide cache folders:
//
//   - Windows: FOLDERID_LocalAppData (C:\Users\<User>\AppData\Local)
//   - Linux/BSDs: ${XDG_CACHE_HOME} (${HOME}/.cache)
//   - MacOSX: "${HOME}/Library/Caches"
//
// On Windows, the folders are retrieved by SHGetKnownFolderPath and
// the environment variables (%PROGRAMDATA%, %APPDATA% and %LOCALAPPDATA%) are used as fallback.
// On macOS, XDG Base Directory folders are used if ConfigDir.XDG is true.
// Inside snap, ${SNAP_REAL_HOME}/.config is used as the user wide folder and
// ${HOME}/.config (snap private folder) is returned as the Confined folder.
//
// configdir returns paths inside the above folders.

package configdir

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

type ConfigType int

const (
	System ConfigType = iota
	Global
	All
	Existing
	Local
	Cache
	// Confined is a user level folder inside a sandbox (e.g. snap's $HOME/snap/<name>/<revision>/.config).
	// It is returned after Global when the application runs confined.
	Confined
)

const (
	defaultFileMode fs.FileMode = 0644
	defaultDirMode  fs.FileMode = 0755
)

// Config represents each folder
type Config struct {
	Path string
	Type ConfigType
	// FileMode is the permission of files created by Create and WriteFile (default: 0644).
	// If it is set, the permission of existing files is also changed when writing.
	FileMode fs.FileMode
	// DirMode is the permission of folders created by this package (default: 0755).
	DirMode fs.FileMode
}

func (c Config) fileMode() fs.FileMode {
	if c.FileMode != 0 {
		return c.FileMode
	}
	return defaultFileMode
}

func (c Config) dirMode() fs.FileMode {
	if c.DirMode != 0 {
		return c.DirMode
	}
	return defaultDirMode
}

// join returns the path of fileName inside c. fileName must be a local path
// (see filepath.IsLocal) to prevent accessing outside of the folder, such as "../secret".
func (c Config) join(op, fileName string) (string, error) {
	if !filepath.IsLocal(fileName) {
		return "", &fs.PathError{Op: op, Path: fileName, Err: fs.ErrInvalid}
	}
	return filepath.Join(c.Path, fileName), nil
}

func (c Config) Open(fileName string) (*os.File, error) {
	path, err := c.join("open", fileName)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

func (c Config) Create(fileName string) (*os.File, error) {
	path, err := c.join("create", fileName)
	if err != nil {
		return nil, err
	}
	if err := c.CreateParentDir(fileName); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, c.fileMode())
	if err != nil {
		return nil, err
	}
	if c.FileMode != 0 {
		if err := file.Chmod(c.FileMode); err != nil {
			file.Close()
			return nil, err
		}
	}
	return file, nil
}

func (c Config) ReadFile(fileName string) ([]byte, error) {
	path, err := c.join("read", fileName)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

// CreateParentDir creates the parent directory of fileName inside c. fileName
// is a relative path inside c, containing zero or more path separators.
func (c Config) CreateParentDir(fileName string) error {
	path, err := c.join("mkdir", fileName)
	if err != nil {
		return err
	}
	return os.MkdirAll(filepath.Dir(path), c.dirMode())
}

func (c Config) WriteFile(fileName string, data []byte) error {
	path, err := c.join("write", fileName)
	if err != nil {
		return err
	}
	if err := c.CreateParentDir(fileName); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, c.fileMode()); err != nil {
		return err
	}
	if c.FileMode != 0 {
		return os.Chmod(path, c.FileMode)
	}
	return nil
}

func (c Config) MkdirAll() error {
	return os.MkdirAll(c.Path, c.dirMode())
}

func (c Config) Exists(fileName string) bool {
	path, err := c.join("stat", fileName)
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return !errors.Is(err, fs.ErrNotExist)
}

// CheckPermission reports whether fileName and folders between c.Path and fileName
// have the permission of FileMode and DirMode.
func (c Config) CheckPermission(fileName string) (bool, error) {
	ok := true
	err := c.walkPermission(fileName, func(path string, info fs.FileInfo, mode fs.FileMode) error {
		if info.Mode().Perm() != mode {
			ok = false
		}
		return nil
	})
	return ok, err
}

// FixPermission changes the permission of fileName and folders between c.Path and fileName
// to FileMode and DirMode.
func (c Config) FixPermission(fileName string) error {
	return c.walkPermission(fileName, func(path string, info fs.FileInfo, mode fs.FileMode) error {
		if info.Mode().Perm() != mode {
			return os.Chmod(path, mode)
		}
		return nil
	})
}

func (c Config) walkPermission(fileName string, fn func(path string, info fs.FileInfo, mode fs.FileMode) error) error {
	path, err := c.join("chmod", fileName)
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if err := fn(path, info, c.fileMode()); err != nil {
		return err
	}
	root := filepath.Clean(c.Path)
	for dir := filepath.Dir(path); len(dir) >= len(root); dir = filepath.Dir(dir) {
		info, err := os.Stat(dir)
		if err != nil {
			return err
		}
		if err := fn(dir, info, c.dirMode()); err != nil {
			return err
		}
		if dir == root {
			break
		}
	}
	return nil
}

// FS returns an fs.FS rooted at the folder.
func (c Config) FS() fs.FS {
	return os.DirFS(c.Path)
}

// ConfigDir keeps setting for querying folders.
type ConfigDir struct {
	VendorName      string
	ApplicationName string
	LocalPath       string
	// XDG uses XDG Base Directory Specification on macOS too
	// (${XDG_CONFIG_HOME}, ${XDG_CONFIG_DIRS} and ${XDG_CACHE_HOME}).
	// It is always used on Linux/BSDs and ignored on Windows.
	XDG bool
	// FileMode and DirMode are passed to Config. See Config.
	FileMode fs.FileMode
	DirMode  fs.FileMode
}

func New(vendorName, applicationName string) ConfigDir {
	return ConfigDir{
		VendorName:      vendorName,
		ApplicationName: applicationName,
	}
}

func (c ConfigDir) joinPath(root string) string {
	if c.VendorName != "" && hasVendorName {
		return filepath.Join(root, c.VendorName, c.ApplicationName)
	}
	return filepath.Join(root, c.ApplicationName)
}

type folders struct {
	system   []string
	global   string
	confined string
	cache    string
}

func (c ConfigDir) folders() folders {
	if c.XDG && runtime.GOOS == "darwin" {
		system, global, cache := xdgFolders(os.Getenv("HOME"))
		return folders{system: system, global: global, cache: cache}
	}
	return folders{
		system:   systemSettingFolders,
		global:   globalSettingFolder,
		confined: confinedSettingFolder,
		cache:    cacheFolder,
	}
}

func (c ConfigDir) newConfig(path string, configType ConfigType) *Config {
	return &Config{
		Path:     path,
		Type:     configType,
		FileMode: c.FileMode,
		DirMode:  c.DirMode,
	}
}

// QueryFolders returns folders in priority order (local, global, confined, system).
//
// Global returns the global folder and the confined folder (if running in a sandbox like snap).
// Existing returns folders that exist and are accessible.
func (c ConfigDir) QueryFolders(configType ConfigType) []*Config {
	if configType == Cache {
		return []*Config{c.QueryCacheFolder()}
	}
	f := c.folders()
	var result []*Config
	if c.LocalPath != "" && configType != System && configType != Global && configType != Confined {
		result = append(result, c.newConfig(c.LocalPath, Local))
	}
	if configType != System && configType != Local && configType != Confined {
		result = append(result, c.newConfig(c.joinPath(f.global), Global))
	}
	if f.confined != "" && configType != System && configType != Local {
		result = append(result, c.newConfig(c.joinPath(f.confined), Confined))
	}
	if configType != Global && configType != Local && configType != Confined {
		for _, root := range f.system {
			result = append(result, c.newConfig(c.joinPath(root), System))
		}
	}
	if configType != Existing {
		return result
	}
	var existing []*Config
	for _, entry := range result {
		if _, err := os.Stat(entry.Path); err == nil {
			existing = append(existing, entry)
		}
	}
	return existing
}

func (c ConfigDir) QueryFolderContainsFile(fileName string) *Config {
	configs := c.QueryFolders(Existing)
	for _, config := range configs {
		if path, err := config.join("stat", fileName); err == nil {
			if _, err := os.Stat(path); err == nil {
				return config
			}
		}
	}
	return nil
}

func (c ConfigDir) QueryCacheFolder() *Config {
	return c.newConfig(c.joinPath(c.folders().cache), Cache)
}

// FS returns a read-only fs.FS that looks up files in the same order as
// QueryFolderContainsFile (local, user, system). Each name is resolved
// in the first folder that contains it; directory listings are not merged.
func (c ConfigDir) FS() fs.FS {
	var layers layeredFS
	for _, config := range c.QueryFolders(All) {
		layers = append(layers, config.FS())
	}
	return layers
}

// layeredFS opens each name from the first underlying fs.FS that has it.
type layeredFS []fs.FS

func (l layeredFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	for _, fsys := range l {
		file, err := fsys.Open(name)
		if err == nil {
			return file, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}
