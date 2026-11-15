package configdir

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"testing/fstest"
)

func TestExeResourceFolder(t *testing.T) {
	root := filepath.FromSlash("/opt")
	tests := []struct {
		name string
		goos string
		exe  string
		want string
	}{
		{"prefix", "linux", filepath.Join(root, "usr", "bin", "app"), filepath.Join(root, "usr", "share", "myapp")},
		{"homebrew", "darwin", filepath.Join(root, "Cellar", "app", "1.0", "bin", "app"), filepath.Join(root, "Cellar", "app", "1.0", "share", "myapp")},
		{"bundle", "darwin", filepath.Join(root, "App.app", "Contents", "MacOS", "app"), filepath.Join(root, "App.app", "Contents", "Resources")},
		{"bundle layout on linux", "linux", filepath.Join(root, "App.app", "Contents", "MacOS", "app"), ""},
		{"not in bin", "linux", filepath.Join(root, "app", "app"), ""},
		{"windows", "windows", filepath.Join(root, "App", "bin", "app.exe"), filepath.Join(root, "App", "bin")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := exeResourceFolder(tt.goos, tt.exe, "myapp"); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// setupResource creates <root>/prefix/bin/app and returns root (symbolic links resolved).
func setupResource(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mkdir := func(path string) {
		t.Helper()
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatal(err)
		}
	}
	mkdir(filepath.Join(root, "prefix", "bin"))
	if err := os.WriteFile(filepath.Join(root, "prefix", "bin", "app"), nil, 0755); err != nil {
		t.Fatal(err)
	}
	saved := executable
	executable = func() (string, error) { return filepath.Join(root, "prefix", "bin", "app"), nil }
	t.Cleanup(func() { executable = saved })
	return root
}

func resourcePaths(configs []*Config) (result []string) {
	for _, config := range configs {
		if config.Type != Resource {
			panic("type should be Resource")
		}
		result = append(result, config.Path)
	}
	return
}

func TestQueryResourceFolders(t *testing.T) {
	root := setupResource(t)
	dev := filepath.Join(root, "dev")
	exeShare := filepath.Join(root, "prefix", "share", "app")
	data1 := filepath.Join(root, "data1")
	os.MkdirAll(dev, 0755)
	os.MkdirAll(exeShare, 0755)
	os.MkdirAll(filepath.Join(data1, "app"), 0755)
	// data2 doesn't exist, prefix/share is the same as the executable based folder
	t.Setenv("XDG_DATA_DIRS", data1+":"+filepath.Join(root, "data2")+":"+filepath.Join(root, "prefix", "share"))

	c := New("vendor", "app")
	c.ResourcePath = dev
	c.XDG = true
	got := resourcePaths(c.QueryResourceFolders())

	want := []string{dev}
	if runtime.GOOS != "windows" {
		want = append(want, exeShare, filepath.Join(data1, "app"))
	} else {
		want = append(want, filepath.Join(root, "prefix", "bin"))
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestQueryResourceFoldersResolvesSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symbolic links requires privilege on Windows")
	}
	root := setupResource(t)
	os.MkdirAll(filepath.Join(root, "prefix", "share", "app"), 0755)
	// e.g. Homebrew: /opt/homebrew/bin/app → /opt/homebrew/Cellar/app/1.0/bin/app
	os.MkdirAll(filepath.Join(root, "link", "bin"), 0755)
	link := filepath.Join(root, "link", "bin", "app")
	if err := os.Symlink(filepath.Join(root, "prefix", "bin", "app"), link); err != nil {
		t.Fatal(err)
	}
	executable = func() (string, error) { return link, nil }
	t.Setenv("XDG_DATA_DIRS", filepath.Join(root, "none"))

	got := resourcePaths(New("", "app").QueryResourceFolders())
	if want := filepath.Join(root, "prefix", "share", "app"); len(got) != 1 || got[0] != want {
		t.Errorf("got %v, want [%s]", got, want)
	}
}

func TestResourceFS(t *testing.T) {
	root := setupResource(t)
	t.Setenv("XDG_DATA_DIRS", filepath.Join(root, "none"))
	c := New("", "app")
	c.ResourcePath = filepath.Join(root, "dev")
	os.MkdirAll(c.ResourcePath, 0755)
	os.WriteFile(filepath.Join(c.ResourcePath, "both.txt"), []byte("folder"), 0644)

	embedded := fstest.MapFS{
		"both.txt":     {Data: []byte("embedded")},
		"embedded.txt": {Data: []byte("embedded")},
	}
	fsys := c.ResourceFS(embedded)
	for name, want := range map[string]string{"both.txt": "folder", "embedded.txt": "embedded"} {
		got, err := fs.ReadFile(fsys, name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if string(got) != want {
			t.Errorf("%s: got %q, want %q", name, got, want)
		}
	}
	if _, err := fs.ReadFile(fsys, "missing.txt"); err == nil {
		t.Error("missing.txt: want error")
	}
}
