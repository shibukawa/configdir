//go:build !windows && !darwin

package configdir

import (
	"slices"
	"testing"
)

func TestUnixFolders(t *testing.T) {
	tests := []struct {
		name         string
		env          map[string]string
		wantSystem   []string
		wantGlobal   string
		wantConfined string
		wantCache    string
	}{
		{
			name:       "default",
			env:        map[string]string{"HOME": "/home/user"},
			wantSystem: []string{"/etc/xdg"},
			wantGlobal: "/home/user/.config",
			wantCache:  "/home/user/.cache",
		},
		{
			name: "xdg",
			env: map[string]string{
				"HOME":            "/home/user",
				"XDG_CONFIG_HOME": "/xdg/config",
				"XDG_CONFIG_DIRS": "/etc/a:/etc/b",
				"XDG_CACHE_HOME":  "/xdg/cache",
			},
			wantSystem: []string{"/etc/a", "/etc/b"},
			wantGlobal: "/xdg/config",
			wantCache:  "/xdg/cache",
		},
		{
			name:         "snap",
			env:          map[string]string{"HOME": "/home/user/snap/app/x1", "SNAP_REAL_HOME": "/home/user"},
			wantSystem:   []string{"/etc/xdg"},
			wantGlobal:   "/home/user/.config",
			wantConfined: "/home/user/snap/app/x1/.config",
			wantCache:    "/home/user/snap/app/x1/.cache",
		},
		{
			name:       "snap with same home",
			env:        map[string]string{"HOME": "/home/user", "SNAP_REAL_HOME": "/home/user"},
			wantSystem: []string{"/etc/xdg"},
			wantGlobal: "/home/user/.config",
			wantCache:  "/home/user/.cache",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, key := range []string{"HOME", "SNAP_REAL_HOME", "XDG_CONFIG_HOME", "XDG_CONFIG_DIRS", "XDG_CACHE_HOME"} {
				t.Setenv(key, tt.env[key])
			}
			system, global, confined, cache := unixFolders()
			if !slices.Equal(system, tt.wantSystem) {
				t.Errorf("system: got %v, want %v", system, tt.wantSystem)
			}
			if global != tt.wantGlobal {
				t.Errorf("global: got %q, want %q", global, tt.wantGlobal)
			}
			if confined != tt.wantConfined {
				t.Errorf("confined: got %q, want %q", confined, tt.wantConfined)
			}
			if cache != tt.wantCache {
				t.Errorf("cache: got %q, want %q", cache, tt.wantCache)
			}
		})
	}
}
