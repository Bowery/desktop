// Copyright 2014 Bowery, Inc.
package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Bowery/gopackages/sys"
)

var (
	pluginManager *PluginManager
	PluginDir     = filepath.Join(os.Getenv(sys.HomeVar), ".bowery", "plugins")
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
	plugin.IsEnabled = true

	return plugin, nil
}

// NewPluginManager creates a PluginManager.
func NewPluginManager() *PluginManager {
	// todo(steve): read through plugin dir and AddPlugin's.
	plugins := make([]*Plugin, 0)

	return &PluginManager{
		Plugins: plugins,
		Event:   make(chan *PluginEvent),
		Error:   make(chan *PluginError),
	}
}

func SetPluginManager() {
	pluginManager = NewPluginManager()
}

func AddPlugin(plugin *Plugin) {
	pluginManager.AddPlugin(plugin)
}

// AddPlugin adds a new Plugin.
func (pm *PluginManager) AddPlugin(plugin *Plugin) {
	pm.Plugins = append(pm.Plugins, plugin)
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

func UpdatePlugin(name string, isEnabled bool) error {
	return pluginManager.UpdatePlugin(name, isEnabled)
}

// UpdatePlugin updates a Plugin.
func (pm *PluginManager) UpdatePlugin(name string, isEnabled bool) error {
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

	pm.Plugins[index].IsEnabled = isEnabled
	log.Println(pm.Plugins)
	return nil
}

// StartPluginListener creates a new plugin manager and
// listens for events.
func StartPluginListener() {
	if pluginManager == nil {
		SetPluginManager()
	}

	// On Event and Error events, execute commands for
	// plugins that have appropriate handlers.
	for {
		select {
		case ev := <-pluginManager.Event:
			log.Println(fmt.Sprintf("plugin event: %s", ev.Type))
			for _, plugin := range pluginManager.Plugins {
				if plugin.IsEnabled {
					if command := plugin.Hooks[ev.Type]; command != "" {
						executeHook(plugin.Name, ev.FilePath, ev.AppDir, command)
					}
				}
			}
		case err := <-pluginManager.Error:
			handlePluginError(err.Plugin.Name, err.Error)
		}
	}
}

// executeHook runs the specified command and returns the
// resulting output.
func executeHook(name, path, dir, command string) {
	log.Println("plugin execute:", fmt.Sprintf("%s: `%s`", name, command))

	var (
		vars []string
		cmds []string
	)
	args := strings.Split(command, " ")
	env := os.Environ()

	// Separate env vars and the cmd.
	for i, arg := range args {
		if strings.Contains(arg, "=") {
			vars = args[:i+1]
		} else {
			cmds = args[i:]
			break
		}
	}

	// Update existing env vars.
	for i, v := range env {
		envlist := strings.SplitN(v, "=", 2)

		for n, arg := range vars {
			arglist := strings.SplitN(arg, "=", 2)

			if arglist[0] == envlist[0] {
				env[i] = arg
				vars[n] = ""
				break
			}
		}
	}

	// Add new env vars.
	for _, arg := range vars {
		if arg != "" {
			env = append(env, arg)
		}
	}

	// Set ENV for hook. The hook will take on the current
	// environment, but will be updated information about
	// the application and files.
	cmd := exec.Command(cmds[0], cmds[1:]...)
	env = append(env, fmt.Sprintf("APP_DIR=%s", dir))
	env = append(env, fmt.Sprintf("FILE_AFFECTED=%s", path))
	cmd.Env = env
	cmd.Dir = filepath.Join(PluginDir, name)
	data, err := cmd.CombinedOutput()
	if err != nil {
		handlePluginError(name, err)
		return
	}

	// debugging
	log.Println(string(data))
}

// handlePluginError handles plugin errors that may occur when loading
// and preparing a plugin, or when executing a plugin's hook.
func handlePluginError(name string, err error) {
	// todo(steve): shoot this down the wire.
	log.Println("plugin error:", fmt.Sprintf("%s: `%s`", name, err.Error()))
}

// EmitPluginEvent creates a new PluginEvent and sends it
// to the pluginManager Event channel.
func EmitPluginEvent(typ, path, dir string) {
	// todo(steve): handle error
	pluginManager.Event <- &PluginEvent{
		Type:     typ,
		FilePath: path,
		AppDir:   dir,
	}
}
