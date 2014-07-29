// Copyright 2014 Bowery, Inc.
package plugin

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Bowery/gopackages/sys"
)

var (
	pluginManager *PluginManager
	pluginDir     = filepath.Join(os.Getenv(sys.HomeVar), ".bowery", "plugins")
)

// Create plugin dir.
func init() {
	if err := os.MkdirAll(pluginDir, os.ModePerm|os.ModeDir); err != nil {
		panic(err)
	}
}

// NewPlugin creates a new plugin.
func NewPlugin(pathToPlugin string) (*Plugin, error) {
	// Read plugin config.
	data, err := ioutil.ReadFile(filepath.Join(pathToPlugin, "plugin.json"))
	if err != nil {
		// todo(steve): handle case where plugin.json is an invalid file.
		// and also add validation of the file.
		return nil, err
	}

	// Unmarshal plugin config.
	plugin := &Plugin{}
	json.Unmarshal(data, &plugin)

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

// LoadPlugin looks through the pluginDir and loads the plugins.
func (pm *PluginManager) LoadPlugins() error {
	// Get contents of plugindir.
	files, err := ioutil.ReadDir(pluginDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		plugin, err := NewPlugin(filepath.Join(pluginDir, file.Name()))
		if err != nil {
			continue
		}

		pm.AddPlugin(plugin)
		log.Println(fmt.Sprintf("Loaded plugin: `%s`", plugin.Name))
	}

	return nil
}

// AddPlugin adds a new Plugin.
func (pm *PluginManager) AddPlugin(plugin *Plugin) {
	pm.Plugins = append(pm.Plugins, plugin)
}

// RemovePlugin removes a Plugin.
func (pm *PluginManager) RemovePlugin(plugin *Plugin) error {
	// todo(steve)
	return nil
}

// UpdatePlugin updates a Plugin.
func (pm *PluginManager) UpdatePlugin(plugin *Plugin) error {
	// todo(steve)
	return nil
}

// StartPluginListener creates a new plugin manager and
// listens for events.
func StartPluginListener() {
	if pluginManager == nil {
		pluginManager = NewPluginManager()
	}

	// Load existing plugins.
	if err := pluginManager.LoadPlugins(); err != nil {
		// todo(steve): do something with error
		log.Println(err)
	}

	// On Event and Error events, execute commands for
	// plugins that have appropriate handlers.
	for {
		select {
		case ev := <-pluginManager.Event:
			log.Println(fmt.Sprintf("plugin event: %s", ev.Type))
			for _, plugin := range pluginManager.Plugins {
				if command := plugin.Hooks[ev.Type]; command != "" {
					executeHook(plugin.Name, ev.FilePath, ev.AppDir, command)
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
	cmd.Dir = filepath.Join(pluginDir, name)
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
