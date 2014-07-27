// Copyright 2014 Bowery, Inc.
package main

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

// Plugin defines the properties and event handlers
// of a plugin.
type Plugin struct {
	Name   string
	Events map[string]string
}

// PluginManager manages all of the plugins as well as
// channels for events and errors.
type PluginManager struct {
	Plugins []*Plugin
	Event   chan *Event
	Error   chan error
}

// Event describes an application event along with
// associated data.
type Event struct {
	// The type of event (e.g. after-restart, before-update)
	Type string

	// The path of the file that has been changed
	Path string

	// The stdout of the command ran.
	Stdout string

	// The stderr of the command ran.
	Stderr string
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
		Event:   make(chan *Event),
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

	// On Event and Error execute commands
	for {
		select {
		case ev := <-pluginManager.Event:
			for _, plugin := range pluginManager.Plugins {
				if command := plugin.Events[ev.Type]; command != "" {
					parts := strings.Fields(command)
					cmd := exec.Command(parts[0], parts[1:len(parts)]...)
					cmd.Stdout = os.Stdout // For debugging.
					cmd.Run()
				}
			}
		case err := <-pluginManager.Error:
			// todo(steve): handle error
			log.Println(err)
		}
	}
}
