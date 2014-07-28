// Copyright 2014 Bowery, Inc.
package plugin

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

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
		Error:   make(chan error),
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
				if command := plugin.Events[ev.Type]; command != "" {
					cmd := ParseCmd(command, nil)
					cmd.Stdout = os.Stdout // For debugging.
					cmd.Run()
				}
			}
		case err := <-pluginManager.Error:
			// todo(steve): handle error.
			log.Println(err, "")
		}
	}
}

// EmitPluginEvent creates a new PluginEvent and sends it
// to the pluginManager Event channel.
func EmitPluginEvent(typ, path string) {
	pluginManager.Event <- &PluginEvent{
		Type: typ,
		Path: path,
	}
}
