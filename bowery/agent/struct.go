// Copyright 2014 Bowery, Inc.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Bowery/gopackages/sys"
)

// Application defines an application.
type Application struct {
	// Unique identifier.
	ID string `json:"id"`

	// Init command. Ran on start and in background.
	Init string `json:"init,omitempty"`

	// Existing init command.
	InitCmd *exec.Cmd `json:"initCmd,omitempty"`

	// Build command. Ran prior to test.
	Build string `json:"build,omitempty"`

	// Existing build command.
	BuildCmd *exec.Cmd `json:"buildCmd,omitempty"`

	// Test command. Ran prior to start.
	Test string `json:"test,omitempty"`

	// Existing test command.
	TestCmd *exec.Cmd `json:"testCmd,omitempty"`

	// Start command. Ran in the background.
	Start string `json:"start,omitempty"`

	// Existing start command.
	StartCmd *exec.Cmd `json:"startCmd,omitempty"`

	// The location of the application's code.
	Path string `json:"path,omitempty"`

	// Commands.
	CmdStrs [4]string `json:"cmdStrs,omitempty"`

	// Enabled plugins: name@version.
	EnabledPlugins []string `json:"enabledPlugins,omitempty"`

	// Plugin processes. Maps a plugin to background and init
	// process pids.
	PluginProcesses map[string]map[string]int `json:"pluginProcesses,omitempty"`

	// State of process (e.g. building, testing, running, etc.)
	State string `json:"processState,omitempty"`

	// OutputWriter for stdout.
	StdoutWriter *OutputWriter `json:"stdoutWriter,-"`

	// OutputWriter for stderr.
	StderrWriter *OutputWriter `json:"stderrWriter,-"`
}

// NewApplication creates a new Application. Validates contents
// and determines target path. Returns a pointer to an Application.
func NewApplication(id, init, build, test, start, path string) (*Application, error) {
	root := ""
	pathList := strings.Split(path, "::")
	if len(pathList) == 2 {
		root = pathList[1]
		if len(root) > 0 && root[0] == '~' {
			root = filepath.Join(os.Getenv(sys.HomeVar), string(root[1:]))
		}
		if (len(root) > 0 && filepath.Separator == '/' && root[0] != '/') ||
			(filepath.Separator != '/' && filepath.VolumeName(root) == "") {
			root = filepath.Join(HomeDir, root)
		}
	} else {
		root = pathList[0]
	}
	if err := os.MkdirAll(root, os.ModePerm|os.ModeDir); err != nil {
		return nil, err
	}

	// Create stdout and stderr writers
	outputPath := filepath.Join(os.Getenv(sys.HomeVar), ".bowery", "log")
	stdoutWriter, err := NewOutputWriter(filepath.Join(outputPath, fmt.Sprintf("%s-stdout.log", id)))
	if err != nil {
		return nil, err
	}
	stderrWriter, err := NewOutputWriter(filepath.Join(outputPath, fmt.Sprintf("%s-stderr.log", id)))
	if err != nil {
		return nil, err
	}

	app := &Application{
		ID:           id,
		Init:         init,
		Build:        build,
		Test:         test,
		Start:        start,
		CmdStrs:      [4]string{},
		Path:         root,
		StdoutWriter: stdoutWriter,
		StderrWriter: stderrWriter,
	}

	return app, nil
}
