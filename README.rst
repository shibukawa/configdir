configdir for Golang
=====================

Multi platform library of configuration directory for Golang.

This library helps to get regular directories for configuration files or cache files that matches target operationg system's convention.

It assumes the following folders are standard paths of each environment:

.. list-table::
   :header-rows: 1

   - * 
     * Windows:
     * Linux/BSDs:
     * MacOSX:
   - * System level configuration folder
     * ``%PROGRAMDATA%`` (``C:\\ProgramData``)
     * ``${XDG_CONFIG_DIRS}`` (``/etc/xdg``)
     * ``/Library/Application Support``
   - * User level configuration folder
     * ``%APPDATA%`` (``C:\\Users\\<User>\\AppData\\Roaming``)
     * ``${XDG_CONFIG_HOME}`` (``${HOME}/.config``)
     * ``${HOME}/Library/Application Support``
   - * User wide cache folder
     * ``%LOCALAPPDATA%`` ``(C:\\Users\\<User>\\AppData\\Local)``
     * ``${XDG_CACHE_HOME}`` (``${HOME}/.cache``)
     * ``${HOME}/Library/Caches``

Notes:

* Windows: folders are retrieved by ``SHGetKnownFolderPath`` (``FOLDERID_ProgramData``, ``FOLDERID_RoamingAppData``, ``FOLDERID_LocalAppData``). The environment variables are used only when the API fails.
* MacOSX: set ``ConfigDir.XDG = true`` to use XDG Base Directory folders (same as Linux/BSDs). It is useful for CLI tools.
* snap: when ``${SNAP_REAL_HOME}`` differs from ``${HOME}``, ``${SNAP_REAL_HOME}/.config`` is used as the user level folder, and ``${HOME}/.config`` (the snap private folder) is returned as a ``Confined`` folder right after it. Accessing the real home folder requires the ``personal-files`` interface.

Installation
------------

.. code-block:: bash

   go get github.com/shibukawa/configdir

Requires Go 1.26 or later.

Examples
------------

Getting Configuration
~~~~~~~~~~~~~~~~~~~~~~~~

``configdir.ConfigDir.QueryFolderContainsFile()`` searches files in the following order:

* Local path (if you add the path via LocalPath parameter)
* User level configuration folder(e.g. ``$HOME/.config/<vendor-name>/<application-name>/setting.json`` in Linux)
* System level configuration folder(e.g. ``/etc/xdg/<vendor-name>/<application-name>/setting.json`` in Linux)

``configdir.Config`` provides some convenient methods(``ReadFile``, ``WriteFile`` and so on).
File names must be local paths inside the folder (see ``filepath.IsLocal``). Names like ``../secret`` or absolute paths are rejected with ``fs.ErrInvalid``.

.. code-block:: go

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

Using fs.FS
~~~~~~~~~~~~~~~~~~~~~~

``configdir.ConfigDir.FS()`` returns a read-only ``fs.FS`` that resolves each file in the same order as ``QueryFolderContainsFile()``.
``configdir.Config.FS()`` returns an ``fs.FS`` rooted at a single folder.

.. code-block:: go

   configDirs := configdir.New("vendor-name", "application-name")
   configDirs.LocalPath, _ = filepath.Abs(".")
   data, err := fs.ReadFile(configDirs.FS(), "setting.json")

Reading Installed Resources
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

``configdir.ConfigDir.QueryResourceFolders()`` returns read-only folders that contain files installed with the application (default settings, templates, assets and so on).
The install location differs by package manager, so it is detected from the executable path (symbolic links are resolved):

.. list-table::
   :header-rows: 1

   - * Executable
     * Resource folder
   - * ``<prefix>/bin/app`` (``/usr``, ``/usr/local``, Homebrew, snap, Flatpak, AppImage...)
     * ``<prefix>/share/<application-name>``
   - * ``X.app/Contents/MacOS/app`` (macOS app bundle)
     * ``X.app/Contents/Resources``
   - * ``<dir>\app.exe`` (Windows)
     * ``<dir>``

On Linux/BSDs (and macOS with ``XDG = true``), ``${XDG_DATA_DIRS}/<application-name>`` (``/usr/local/share``, ``/usr/share``) is searched next.
``ResourcePath`` has the highest priority. It is useful for development because ``go run`` and ``go test`` build the executable into a temporary folder.

``ResourceFS()`` returns an ``fs.FS`` over the folders. ``embed.FS`` can be passed as fallback:

.. code-block:: go

   //go:embed resources
   var embedded embed.FS

   configDirs := configdir.New("vendor-name", "application-name")
   configDirs.ResourcePath = os.Getenv("APP_RESOURCE_PATH") // optional
   resources, _ := fs.Sub(embedded, "resources")
   data, err := fs.ReadFile(configDirs.ResourceFS(resources), "template.html")

Write Configuration
~~~~~~~~~~~~~~~~~~~~~~

When storing configuration, get configuration folder by using ``configdir.ConfigDir.QueryFolders()`` method.

.. code-block:: go

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

Writing from snap
~~~~~~~~~~~~~~~~~~~~~~

``QueryFolders(configdir.Global)`` returns the real home folder first and the confined folder next. Fall back to the next folder when writing fails:

.. code-block:: go

   for _, folder := range configDirs.QueryFolders(configdir.Global) {
       if err := folder.WriteFile("setting.json", data); err == nil {
           break
       }
   }

File Permission
~~~~~~~~~~~~~~~~~~~~~~

Configuration files sometimes contain secrets. ``FileMode`` and ``DirMode`` (default: ``0644`` and ``0755``) are used by ``Create``, ``WriteFile``, ``MkdirAll`` and so on.
When ``FileMode`` is set, the permission of an existing file is also changed on writing.

.. code-block:: go

   configDirs := configdir.New("vendor-name", "application-name")
   configDirs.FileMode = 0600
   configDirs.DirMode = 0700

   folder := configDirs.QueryFolders(configdir.Global)[0]
   folder.WriteFile("credential.json", data)

   // check and fix permission of files created by other tools
   if ok, _ := folder.CheckPermission("credential.json"); !ok {
       folder.FixPermission("credential.json")
   }

Getting Cache Folder
~~~~~~~~~~~~~~~~~~~~~~

It is similar to the above example, but returns cache folder.

.. code-block:: go

   configDirs := configdir.New("vendor-name", "application-name")
   cache := configDirs.QueryCacheFolder()

   resp, err := http.Get("http://examples.com/sdk.zip")
   if err != nil {
       log.Fatal(err)
   }
   defer resp.Body.Close()
   body, err := io.ReadAll(resp.Body)

   cache.WriteFile("sdk.zip", body)

Document
------------

https://pkg.go.dev/github.com/shibukawa/configdir

License
------------

MIT

