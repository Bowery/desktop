// Copyright 2014 Bowery, Inc.
package plugin

// Plugin defines the properties and event handlers
// of a plugin.
type Plugin struct {
	Name    string
	Author  PluginAuthor
	Website string
	Events  map[string]string
}

// PluginAuther defines the attributes and properties
// of a plugin author.
type PluginAuthor struct {
	Name    string
	Email   string
	Twitter string
	Github  string
}

// PluginManager manages all of the plugins as well as
// channels for events and errors.
type PluginManager struct {
	Plugins []*Plugin
	Event   chan *PluginEvent
	Error   chan error
}

// Event describes an application event along with
// associated data.
type PluginEvent struct {
	// The type of event (e.g. after-restart, before-update)
	Type string

	// The path of the file that has been changed
	Path string

	// The stdout of the command ran.
	Stdout string

	// The stderr of the command ran.
	Stderr string
}
