package configdir

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"testing/fstest"
)

func TestConfigFS(t *testing.T) {
	dir := t.TempDir()
	c := Config{Path: dir, Type: Local}
	files := []string{"setting.json"}
	// On Windows, the ModTime of a folder in its parent's listing may lag behind os.Stat
	// after creating a file in it, and fstest.TestFS reports it as a mismatch.
	if runtime.GOOS != "windows" {
		files = append(files, "sub/setting.json")
	}
	for _, name := range files {
		if err := c.WriteFile(name, []byte("{}")); err != nil {
			t.Fatal(err)
		}
	}
	if err := fstest.TestFS(c.FS(), files...); err != nil {
		t.Fatal(err)
	}
}

func TestConfigDirFS(t *testing.T) {
	local := t.TempDir()
	global := t.TempDir()
	write := func(root, name, data string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	saved := globalSettingFolder
	globalSettingFolder = global
	defer func() { globalSettingFolder = saved }()

	c := New("vendor", "app")
	c.LocalPath = local
	globalApp := c.joinPath(global)
	write(local, "both.txt", "local")
	write(globalApp, "both.txt", "global")
	write(globalApp, "global-only.txt", "global")

	fsys := c.FS()
	for name, want := range map[string]string{
		"both.txt":        "local",
		"global-only.txt": "global",
	} {
		got, err := fs.ReadFile(fsys, name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if string(got) != want {
			t.Errorf("%s: got %q, want %q", name, got, want)
		}
	}
	if _, err := fs.ReadFile(fsys, "missing.txt"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing.txt: got %v, want fs.ErrNotExist", err)
	}
	if _, err := fsys.Open("../escape"); !errors.Is(err, fs.ErrInvalid) {
		t.Errorf("../escape: got %v, want fs.ErrInvalid", err)
	}
}

func TestPermission(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission is not supported")
	}
	dir := t.TempDir()
	c := Config{Path: filepath.Join(dir, "app"), FileMode: 0600, DirMode: 0700}
	if err := c.WriteFile("sub/secret.json", []byte("{}")); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]fs.FileMode{
		filepath.Join(c.Path, "sub", "secret.json"): 0600,
		filepath.Join(c.Path, "sub"):                0700,
		c.Path:                                      0700,
	} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != want {
			t.Errorf("%s: got %o, want %o", path, info.Mode().Perm(), want)
		}
	}
	if ok, err := c.CheckPermission("sub/secret.json"); err != nil || !ok {
		t.Errorf("CheckPermission: got %v, %v; want true", ok, err)
	}

	// loosen permissions, then fix them
	os.Chmod(filepath.Join(c.Path, "sub", "secret.json"), 0644)
	os.Chmod(filepath.Join(c.Path, "sub"), 0755)
	if ok, err := c.CheckPermission("sub/secret.json"); err != nil || ok {
		t.Errorf("CheckPermission: got %v, %v; want false", ok, err)
	}
	if err := c.FixPermission("sub/secret.json"); err != nil {
		t.Fatal(err)
	}
	if ok, err := c.CheckPermission("sub/secret.json"); err != nil || !ok {
		t.Errorf("CheckPermission after fix: got %v, %v; want true", ok, err)
	}
	// parent of c.Path is out of scope
	info, _ := os.Stat(dir)
	if info.Mode().Perm() == 0700 {
		t.Errorf("parent of Config.Path should not be changed")
	}

	// existing file permission is also changed by WriteFile
	os.Chmod(filepath.Join(c.Path, "sub", "secret.json"), 0644)
	if err := c.WriteFile("sub/secret.json", []byte("{}")); err != nil {
		t.Fatal(err)
	}
	info, _ = os.Stat(filepath.Join(c.Path, "sub", "secret.json"))
	if info.Mode().Perm() != 0600 {
		t.Errorf("WriteFile to existing file: got %o, want 0600", info.Mode().Perm())
	}
}

func TestConfinedFolder(t *testing.T) {
	savedGlobal, savedConfined := globalSettingFolder, confinedSettingFolder
	globalSettingFolder, confinedSettingFolder = "/home/user/.config", "/home/user/snap/app/x1/.config"
	defer func() { globalSettingFolder, confinedSettingFolder = savedGlobal, savedConfined }()

	c := New("vendor", "app")
	c.LocalPath = "/local"
	types := func(configs []*Config) (result []ConfigType) {
		for _, config := range configs {
			result = append(result, config.Type)
		}
		return
	}
	all := types(c.QueryFolders(All))
	if len(all) < 3 || all[0] != Local || all[1] != Global || all[2] != Confined {
		t.Errorf("All: got %v", all)
	}
	global := c.QueryFolders(Global)
	if len(global) != 2 || global[0].Type != Global || global[1].Type != Confined {
		t.Errorf("Global: got %v", types(global))
	}
	if confined := c.QueryFolders(Confined); len(confined) != 1 || confined[0].Path != c.joinPath(confinedSettingFolder) {
		t.Errorf("Confined: got %v", confined)
	}
	for _, config := range c.QueryFolders(System) {
		if config.Type != System {
			t.Errorf("System: got %v", config.Type)
		}
	}
}

func TestXDGOnDarwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin only")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_CONFIG_DIRS", "")
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))

	c := New("vendor", "app")
	c.XDG = true
	if got, want := c.QueryFolders(Global)[0].Path, filepath.Join(home, ".config", "vendor", "app"); got != want {
		t.Errorf("Global: got %s, want %s", got, want)
	}
	if got, want := c.QueryFolders(System)[0].Path, filepath.Join("/etc/xdg", "vendor", "app"); got != want {
		t.Errorf("System: got %s, want %s", got, want)
	}
	if got, want := c.QueryCacheFolder().Path, filepath.Join(home, "cache", "vendor", "app"); got != want {
		t.Errorf("Cache: got %s, want %s", got, want)
	}
	c.XDG = false
	if got := c.QueryFolders(Global)[0].Path; got != filepath.Join(globalSettingFolder, "vendor", "app") {
		t.Errorf("Global without XDG: got %s", got)
	}
}

func TestFileModePropagation(t *testing.T) {
	c := New("vendor", "app")
	c.FileMode, c.DirMode = 0600, 0700
	for _, config := range append(c.QueryFolders(All), c.QueryCacheFolder()) {
		if config.FileMode != 0600 || config.DirMode != 0700 {
			t.Errorf("%s: mode is not propagated", config.Path)
		}
	}
}

func TestRejectNonLocalPath(t *testing.T) {
	dir := t.TempDir()
	c := Config{Path: filepath.Join(dir, "app")}
	if err := c.WriteFile("ok.txt", []byte("ok")); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "outside.txt"), []byte("secret"), 0644)

	for _, name := range []string{"../outside.txt", "sub/../../outside.txt", filepath.Join(dir, "outside.txt"), ""} {
		if _, err := c.Open(name); !errors.Is(err, fs.ErrInvalid) {
			t.Errorf("Open(%q): got %v, want fs.ErrInvalid", name, err)
		}
		if _, err := c.ReadFile(name); !errors.Is(err, fs.ErrInvalid) {
			t.Errorf("ReadFile(%q): got %v, want fs.ErrInvalid", name, err)
		}
		if _, err := c.Create(name); !errors.Is(err, fs.ErrInvalid) {
			t.Errorf("Create(%q): got %v, want fs.ErrInvalid", name, err)
		}
		if err := c.WriteFile(name, nil); !errors.Is(err, fs.ErrInvalid) {
			t.Errorf("WriteFile(%q): got %v, want fs.ErrInvalid", name, err)
		}
		if err := c.CreateParentDir(name); !errors.Is(err, fs.ErrInvalid) {
			t.Errorf("CreateParentDir(%q): got %v, want fs.ErrInvalid", name, err)
		}
		if err := c.FixPermission(name); !errors.Is(err, fs.ErrInvalid) {
			t.Errorf("FixPermission(%q): got %v, want fs.ErrInvalid", name, err)
		}
		if c.Exists(name) {
			t.Errorf("Exists(%q): got true", name)
		}
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "outside.txt")); string(got) != "secret" {
		t.Errorf("outside file is modified: %q", got)
	}
	// ".." inside the folder is still allowed
	if _, err := c.ReadFile("sub/../ok.txt"); err != nil {
		t.Errorf("ReadFile(sub/../ok.txt): %v", err)
	}

	cd := New("", "app")
	cd.LocalPath = c.Path
	if got := cd.QueryFolderContainsFile("../outside.txt"); got != nil {
		t.Errorf("QueryFolderContainsFile: got %s", got.Path)
	}
}
