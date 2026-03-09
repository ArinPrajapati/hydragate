package config

import (
	"log/slog"
	"time"

	"hydragate/internal/plugin"

	"github.com/fsnotify/fsnotify"
)

func WatchConfig(configPath string, state *State, pluginRegistry *plugin.PluginRegistry) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Error("failed to create file watcher", "error", err)
		return
	}
	defer watcher.Close()

	err = watcher.Add(configPath)
	if err != nil {
		slog.Error("failed to watch config file", "path", configPath, "error", err)
		return
	}

	slog.Info("config watcher started", "path", configPath)

	debounceTimer := time.NewTimer(0)
	<-debounceTimer.C

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
				debounceTimer.Stop()
				debounceTimer = time.AfterFunc(100*time.Millisecond, func() {
					slog.Info("config file changed, reloading...", "path", configPath)
					if err := Reload(state, configPath, pluginRegistry); err != nil {
						slog.Error("failed to reload config", "error", err)
					} else {
						slog.Info("config reloaded successfully")
					}
				})
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			slog.Error("config watcher error", "error", err)
		}
	}
}
