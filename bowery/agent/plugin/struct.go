// Copyright 2014 Bowery, Inc.
package plugin

// Plugin defines the properties and event handlers
// of a plugin.
type Plugin struct {
	// Name of plugin.
	Name string

	// Author of plugin.
	Author PluginAuthor

	// Hooks and associated handlers.
	Hooks map[string]string

	// Whether the plugin is being used or not.
	IsEnabled bool
}

// PluginAuther defines the attributes and properties
// of a plugin author.
type PluginAuthor struct {
	// e.g. Steve Kaliski
	Name string

	// e.g. steve@bowery.io
	Email string

	// e.g. stevekaliski
	Twitter string

	// e.g. sjkaliski
	Github string
}

// PluginManager manages all of the plugins as well as
// channels for events and errors.
type PluginManager struct {
	// Array of active plugins.
	Plugins []*Plugin

	// PluginEvent channel.
	Event chan *PluginEvent

	// PluginError channel.
	Error chan *PluginError
}

// Event describes a plugin event along with
// associated data.
type PluginEvent struct {
	// The type of event (e.g. after-restart, before-update)
	Type string

	// The path of the file that has been changed.
	FilePath string

	// The directory of the application code.
	AppDir string

	// The stdout of the command ran.
	Stdout string

	// The stderr of the command ran.
	Stderr string
}

// Error describes a plugin error along with
// associated data.
type PluginError struct {
	// The plugin the error came from.
	Plugin *Plugin

	// The error that occured.
	Error error
}
