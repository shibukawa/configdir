package configdir

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestKnownFolderPath(t *testing.T) {
	for env, got := range map[string]string{
		"PROGRAMDATA":  systemSettingFolders[0],
		"APPDATA":      globalSettingFolder,
		"LOCALAPPDATA": cacheFolder,
	} {
		if !filepath.IsAbs(got) {
			t.Errorf("%s: got %q, want absolute path", env, got)
		}
		// On a regular environment, the API and the environment variable point to the same folder.
		if want := os.Getenv(env); want != "" && !strings.EqualFold(filepath.Clean(got), filepath.Clean(want)) {
			t.Errorf("%s: got %q, want %q", env, got, want)
		}
	}
}

func TestKnownFolderPathFallback(t *testing.T) {
	t.Setenv("CONFIGDIR_TEST_FOLDER", `C:\fallback`)
	unknown := &windows.KNOWNFOLDERID{Data1: 0xdeadbeef}
	if got := knownFolderPath(unknown, "CONFIGDIR_TEST_FOLDER"); got != `C:\fallback` {
		t.Errorf("got %q, want fallback to environment variable", got)
	}
}

// Windows supports only the read-only attribute, so the permission check itself can't match
// unix modes. Make sure FileMode/DirMode don't break writing and fixing.
func TestPermissionOnWindows(t *testing.T) {
	c := Config{Path: filepath.Join(t.TempDir(), "app"), FileMode: 0600, DirMode: 0700}
	if err := c.WriteFile("sub/secret.json", []byte("{}")); err != nil {
		t.Fatal(err)
	}
	if err := c.WriteFile("sub/secret.json", []byte("{}")); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	if _, err := c.CheckPermission("sub/secret.json"); err != nil {
		t.Errorf("CheckPermission: %v", err)
	}
	if err := c.FixPermission("sub/secret.json"); err != nil {
		t.Errorf("FixPermission: %v", err)
	}
	if err := c.WriteFile("sub/secret.json", []byte("{}")); err != nil {
		t.Errorf("write after FixPermission: %v", err)
	}
	if data, err := c.ReadFile("sub/secret.json"); err != nil || string(data) != "{}" {
		t.Errorf("ReadFile: %q, %v", data, err)
	}
}
