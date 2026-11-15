package configdir_test

import (
	"encoding/json"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"testing/fstest"

	"github.com/shibukawa/configdir"
)

type Config struct {
	UserName string `json:"user-name"`
}

var DefaultConfig = Config{
	UserName: "baron", // Do you remember BeOS?
}

// Sample for reading configuration
func ExampleConfigDir() {

	var config Config

	configDirs := configdir.New("vendor-name", "application-name")
	// optional: local path has the highest priority
	configDirs.LocalPath, _ = filepath.Abs(".")
	folder := configDirs.QueryFolderContainsFile("setting.json")
	if folder != nil {
		data, _ := folder.ReadFile("setting.json")
		json.Unmarshal(data, &config)
	} else {
		config = DefaultConfig
	}
}

// Sample for reading configuration
func ExampleConfigDir_QueryFolders() {
	configDirs := configdir.New("vendor-name", "application-name")

	var config Config
	data, _ := json.Marshal(&config)

	// Stores to local folder
	folders := configDirs.QueryFolders(configdir.Local)
	folders[0].WriteFile("setting.json", data)

	// Stores to user folder
	folders = configDirs.QueryFolders(configdir.Global)
	folders[0].WriteFile("setting.json", data)

	// Stores to system folder
	folders = configDirs.QueryFolders(configdir.System)
	folders[0].WriteFile("setting.json", data)
}

// Sample for getting cache folder
func ExampleConfigDir_QueryCacheFolder() {
	configDirs := configdir.New("vendor-name", "application-name")
	cache := configDirs.QueryCacheFolder()

	resp, err := http.Get("http://examples.com/sdk.zip")
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)

	cache.WriteFile("sdk.zip", body)
}

// Sample for reading configuration via fs.FS
func ExampleConfigDir_FS() {
	var config Config

	configDirs := configdir.New("vendor-name", "application-name")
	configDirs.LocalPath, _ = filepath.Abs(".")
	data, err := fs.ReadFile(configDirs.FS(), "setting.json")
	if err == nil {
		json.Unmarshal(data, &config)
	} else {
		config = DefaultConfig
	}
}

// Sample for reading files installed with the application
func ExampleConfigDir_ResourceFS() {
	configDirs := configdir.New("vendor-name", "application-name")
	// optional: highest priority (e.g. for go run)
	configDirs.ResourcePath = os.Getenv("APP_RESOURCE_PATH")

	// embedded files can be passed as fallback (e.g. embed.FS)
	embedded := fstest.MapFS{"template.html": {Data: []byte("<html></html>")}}
	data, _ := fs.ReadFile(configDirs.ResourceFS(embedded), "template.html")
	_ = data
}
