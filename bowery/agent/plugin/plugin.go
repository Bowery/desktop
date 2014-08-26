// Copyright 2014 Bowery, Inc.
package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/Bowery/gopackages/sys"
)

var (
	pluginManager *PluginManager
	PluginDir     = filepath.Join(os.Getenv(sys.HomeVar), ".bowery", "plugins")
	LogDir        = filepath.Join(os.Getenv(sys.HomeVar), ".bowery", "log")
)

// Create plugin dir.
func init() {
	if PluginDir == "" {
		filepath.Join(os.Getenv(sys.HomeVar), ".bowery", "plugins")
	}
	if err := os.MkdirAll(PluginDir, os.ModePerm|os.ModeDir); err != nil {
		panic(err)
	}
}

// NewPlugin creates a new plugin.
func NewPlugin(name, hooks string) (*Plugin, error) {
	// Unmarshal plugin config.
	data := []byte(hooks)
	plugin := &Plugin{}
	plugin.Name = name
	json.Unmarshal(data, &plugin.Hooks)
	return plugin, nil
}

// NewPluginManager creates a PluginManager.
func NewPluginManager() *PluginManager {
	plugins := make([]*Plugin, 0)

	return &PluginManager{
		Plugins: plugins,
		Event:   make(chan *PluginEvent),
		Error:   make(chan *PluginError),
	}
}

func SetPluginManager() *PluginManager {
	pluginManager = NewPluginManager()
	return pluginManager
}

func AddPlugin(plugin *Plugin) error {
	return pluginManager.AddPlugin(plugin)
}

// AddPlugin adds a new Plugin.
func (pm *PluginManager) AddPlugin(plugin *Plugin) error {
	// makes sure that when dev-mode is turned on the dev plugins overwrite the old ones
	for i, p := range pm.Plugins {
		if p.Name == plugin.Name {
			pm.Plugins[i] = plugin
			return errors.New("plugin exists")
		}
	}

	pm.Plugins = append(pm.Plugins, plugin)
	return nil
}

// RemovePlugin removes a Plugin.
func RemovePlugin(name string) error {
	return pluginManager.RemovePlugin(name)
}

// RemovePlugin removes a Plugin by name.
func (pm *PluginManager) RemovePlugin(name string) error {
	index := -1
	for i, plugin := range pm.Plugins {
		if plugin.Name == name {
			index = i
			break
		}
	}

	if index == -1 {
		return errors.New("invalid plugin name")
	}

	pm.Plugins = append(pm.Plugins[:index], pm.Plugins[index+1:]...)
	return nil
}

// GetPlugins returns a slice of Plugins.
func GetPlugins() []*Plugin {
	return pluginManager.Plugins
}

func GetPlugin(name string) *Plugin {
	return pluginManager.GetPlugin(name)
}

func (pm *PluginManager) GetPlugin(name string) *Plugin {
	for _, plugin := range pm.Plugins {
		if plugin.Name == name {
			return plugin
		}
	}

	return nil
}

// StartPluginListener creates a new plugin manager and
// listens for events.
func StartPluginListener() {
	if pluginManager == nil {
		SetPluginManager()
	}

	// On Event and Error events, execute commands for
	// plugins that have appropriate handlers and are
	// enabled by the specified application.
	for {
		select {
		case ev := <-pluginManager.Event:
			log.Println(fmt.Sprintf("plugin event: %s", ev.Type))
			for _, plugin := range pluginManager.Plugins {
				for _, ep := range ev.EnabledPlugins {
					if ep == plugin.Name {
						if command := plugin.Hooks[ev.Type]; command != "" {
							if ev.Type == BACKGROUND {
								executeHook(plugin, ev.FilePath, ev.AppDir, command, true)
							} else {
								executeHook(plugin, ev.FilePath, ev.AppDir, command, false)
							}
						}
					}
				}
			}
		}
	}
}

// executeHook runs the specified command and returns the
// resulting output.
func executeHook(plugin *Plugin, path, dir, command string, background bool) {
	name := plugin.Name
	log.Println("plugin execute:", fmt.Sprintf("%s: `%s`", name, command))

	// Set the env for the hook, includes info about the file being modified,
	// and if background the paths for stdio.
	env := map[string]string{
		"APP_DIR":       dir,
		"FILE_AFFECTED": path,
	}
	if background {
		env["STDOUT"] = filepath.Join(LogDir, "stdout.log")
		env["STDERR"] = filepath.Join(LogDir, "stderr.log")
	}

	cmd := sys.NewCommand(command, env)
	cmd.Dir = filepath.Join(PluginDir, name)

	// If it is not a background process, execute immediately
	// and wait for it to complete. If it is a background process
	// pipe the agent's Stdin into the command and run.
	if !background {
		data, err := cmd.CombinedOutput()
		if err != nil {
			handlePluginError(plugin, command, err)
			return
		}

		// debugging
		log.Println(string(data))
	} else {
		// Start the process. If there is an issue starting, alert
		// the client.
		plugin.BackgroundCommand = cmd

		go func() {
			if err := cmd.Start(); err != nil {
				handlePluginError(plugin, command, err)
				return
			}
			if err := cmd.Wait(); err != nil {
				handlePluginError(plugin, command, err)
			}
		}()
	}
}

// handlePluginError emits the values of the plugin error to pluginManager.Error
func handlePluginError(plugin *Plugin, command string, err error) {
	pluginManager.Error <- &PluginError{
		Plugin:  plugin,
		Command: command,
		Error:   err,
	}
}

// EmitPluginEvent creates a new PluginEvent and sends it
// to the pluginManager Event channel.
func EmitPluginEvent(typ, path, dir, id string, enabledPlugins []string) {
	// todo(steve): handle error
	pluginManager.Event <- &PluginEvent{
		Type:           typ,
		FilePath:       path,
		AppDir:         dir,
		Identifier:     id,
		EnabledPlugins: enabledPlugins,
	}
}
