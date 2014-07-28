// Copyright 2014 Bowery, Inc.
// Tests for the Plugin API. Includes loading, adding, updating, and
// removing plugins.
//
// Note(steve): Not completed yet.
package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

var (
	testPlugin = &Plugin{
		Name: "test-plugin",
		Events: map[string]string{
			"after-restart": "echo Restart",
		},
	}
	testPluginManager *PluginManager
)

func init() {
	pluginDir = "plugins"

	data, err := json.Marshal(testPlugin)
	if err != nil {
		panic(err)
	}

	if err := os.MkdirAll(filepath.Join(pluginDir, "test-plugin"), os.ModePerm|os.ModeDir); err != nil {
		panic(err)
	}

	if err = ioutil.WriteFile(filepath.Join(pluginDir, "test-plugin", "plugin.json"), data, 0644); err != nil {
		panic(err)
	}
}

func TestNewPluginManager(t *testing.T) {
	testPluginManager = NewPluginManager()

	if len(testPluginManager.Plugins) > 0 {
		t.Fatal("NewPluginManager created with non-zero quantity of plugins.")
	}
}

func TestLoadPlugins(t *testing.T) {
	if err := testPluginManager.LoadPlugins(); err != nil {
		t.Error(err)
	}

	if len(testPluginManager.Plugins) != 1 {
		t.Fatal("Failed to load plugins.")
	}
}

func TestNewPluginWithNoDirectory(t *testing.T) {
	_, err := NewPlugin(filepath.Join(pluginDir, "invalid-plugin"))
	if err == nil {
		t.Fatal("A new plugin was created without valid plugin.json file.", err)
	}
}

func TestNewPluginWithValidDirectory(t *testing.T) {
	plugin, err := NewPlugin(filepath.Join(pluginDir, "test-plugin"))
	if err != nil {
		t.Error(err)
	}

	if plugin.Name != "test-plugin" || plugin.Events["after-restart"] != "echo Restart" {
		t.Error("Plugin properties not properly set.")
	}

	// cleanup
	os.RemoveAll("plugins")
}
