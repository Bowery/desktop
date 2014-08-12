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

	// Build command. Ran prior to test.
	Build string `json:"build,omitempty"`

	// Test command. Ran prior to start.
	Test string `json:"test,omitempty"`

	// Start command. Ran in the background.
	Start string `json:"start,omitempty"`

	// The location of the application's code.
	Path string `json:"path,omitempty"`

	// Commands.
	CmdStrs [4]string `json:"cmdStrs,omitempty"`

	// Existing background command.
	ExistingCommand *exec.Cmd `json:"existingCommand,omitempty"`

	// Enabled plugins: name@version.
	EnabledPlugins []string `json:"enabledPlugins,omitempty"`

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
	pathList := strings.Split(path, ":")
	if len(pathList) == 2 {
		root = pathList[1]
		if string(root[0]) == "~" {
			root = filepath.Join(os.Getenv(sys.HomeVar), string(root[1:]))
		}
		if string(root[0]) != "/" {
			root = filepath.Join(os.Getenv(sys.HomeVar), root)
		}
		if err := os.RemoveAll(root); err != nil {
			return nil, err
		}
		err := os.MkdirAll(root, os.ModePerm|os.ModeDir)
		if err != nil {
			return nil, err
		}
	} else {
		root = pathList[0]
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
