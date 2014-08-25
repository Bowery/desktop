// Copyright 2013-2014 Bowery, Inc.
// Contains routines to manage the service.
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Bowery/desktop/bowery/agent/plugin"
)

var (
	ImageScriptsDir = filepath.Join(BoweryDir, "image_scripts")
	mutex           sync.Mutex
)

// Proc describes a processes ids and its children.
type Proc struct {
	Pid      int
	Ppid     int
	Children []*Proc
}

// Kill kills the proc and its children.
func (proc *Proc) Kill() error {
	p, err := os.FindProcess(proc.Pid)
	if err != nil {
		return err
	}

	err = p.Kill()
	if err != nil {
		return err
	}

	if proc.Children != nil {
		for _, p := range proc.Children {
			err = p.Kill()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Restart restarts the services processes, the init cmd is only restarted
// if initReset is true. Commands to run are only updated if reset is true.
// A channel is returned and signaled if the commands start or the build fails.
func Restart(app *Application, initReset, reset bool) chan bool {
	plugin.EmitPluginEvent(plugin.BEFORE_APP_RESTART, "", app.Path, app.ID, app.EnabledPlugins)
	mutex.Lock() // Lock here so no other restarts can interfere.
	finish := make(chan bool, 1)
	log.Println(fmt.Sprintf("restart beginning: %s", app.ID))

	init := app.Init
	build := app.Build
	test := app.Test
	start := app.Start
	stdoutWriter := app.StdoutWriter
	stderrWriter := app.StderrWriter

	// Create cmds.
	if reset {
		if !initReset {
			init = app.CmdStrs[0]
		}

		app.CmdStrs = [4]string{init, build, test, start}
	}

	initCmd := parseCmd(app.CmdStrs[0], app.Path, stdoutWriter, stderrWriter)
	buildCmd := parseCmd(app.CmdStrs[1], app.Path, stdoutWriter, stderrWriter)
	testCmd := parseCmd(app.CmdStrs[2], app.Path, stdoutWriter, stderrWriter)
	startCmd := parseCmd(app.CmdStrs[3], app.Path, stdoutWriter, stderrWriter)

	// Kill existing commands.
	err := Kill(app, initReset)
	if err != nil {
		mutex.Unlock()
		finish <- false
		return finish
	}

	app.InitCmd = initCmd
	app.BuildCmd = buildCmd
	app.TestCmd = testCmd
	app.StartCmd = startCmd

	// Run in goroutine so commands can run in the background.
	go func() {
		var wg sync.WaitGroup
		defer wg.Wait()
		defer mutex.Unlock()
		cmds := make([]*exec.Cmd, 0)

		// Get the image_scripts and start them.
		dir, _ := os.Open(ImageScriptsDir)
		if dir != nil {
			infos, _ := dir.Readdir(0)
			if infos != nil {
				for _, info := range infos {
					if info.IsDir() {
						continue
					}

					cmd := parseCmd(filepath.Join(ImageScriptsDir, info.Name()), ImageScriptsDir, stdoutWriter, stderrWriter)
					if cmd != nil {
						err := startProc(cmd, stdoutWriter, stderrWriter)
						if err == nil {
							cmds = append(cmds, cmd)
						}
					}
				}
			}

			dir.Close()
		}

		// Run the build command, only proceed if successful.
		if buildCmd != nil {
			app.State = "building"
			log.Println("Running Build Command")
			err := buildCmd.Run()
			if err != nil {
				stderrWriter.Write([]byte(err.Error() + "\n"))

				Kill(app, initReset)
				finish <- false
				return
			}
			log.Println("Finished Build Command")
		}

		// Start the test command.
		if testCmd != nil {
			app.State = "testing"
			err := startProc(testCmd, stdoutWriter, stderrWriter)
			if err == nil {
				cmds = append(cmds, testCmd)
			}
		}

		// Start the init command if init is set.
		if initReset && initCmd != nil {
			err := startProc(initCmd, stdoutWriter, stderrWriter)
			if err == nil {
				app.InitCmd = initCmd
			}
		}

		// Start the start command.
		if startCmd != nil {
			app.State = "running"
			err := startProc(startCmd, stdoutWriter, stderrWriter)
			if err == nil {
				cmds = append(cmds, startCmd)
			}
		}

		// Signal the start and prepare the wait group to keep tcp open.
		finish <- true
		wg.Add(len(cmds))

		// Wait for the init process to end
		if initReset && app.InitCmd != nil {
			wg.Add(1)

			go func(c *exec.Cmd) {
				waitProc(c, stdoutWriter, stderrWriter)
				wg.Done()
			}(app.InitCmd)
		}

		// Loop the commands and wait for them in parallel.
		for _, cmd := range cmds {
			go func(c *exec.Cmd) {
				waitProc(c, stdoutWriter, stderrWriter)
				wg.Done()
			}(cmd)
		}

		log.Println(fmt.Sprintf("restart completed: %s", app.ID))
	}()

	plugin.EmitPluginEvent(plugin.AFTER_APP_RESTART, "", app.Path, app.ID, app.EnabledPlugins)
	return finish
}

// Kill kills the services processes, the init cmd is only killed if init is true.
// todo(steve): figure out plugins.
func Kill(app *Application, init bool) error {
	if init {
		err := killByCmd(app.InitCmd)
		if err != nil {
			return err
		}
	}

	err := killByCmd(app.BuildCmd)
	if err != nil {
		return err
	}

	err = killByCmd(app.TestCmd)
	if err != nil {
		return err
	}

	return killByCmd(app.StartCmd)
}

// waitProc waits for a process to end and writes errors to tcp.
func waitProc(cmd *exec.Cmd, stdoutWriter, stderrWriter *OutputWriter) {
	err := cmd.Wait()
	if err != nil {
		stderrWriter.Write([]byte(err.Error() + "\n"))
	}
}

// startProc starts a process and writes errors to tcp.
func startProc(cmd *exec.Cmd, stdoutWriter, stderrWriter *OutputWriter) error {
	err := cmd.Start()
	if err != nil {
		stderrWriter.Write([]byte(err.Error() + "\n"))
	}

	return err
}

// killByCmd kills the process tree for a given cmd.
func killByCmd(cmd *exec.Cmd) error {
	if cmd != nil && cmd.Process != nil {
		proc, err := GetPidTree(cmd.Process.Pid)
		if err != nil {
			return err
		}

		return proc.Kill()
	}

	return nil
}

// parseCmd converts a string to a command, connecting stdio to a
// tcp connection.
func parseCmd(command, dir string, stdoutWriter, stderrWriter *OutputWriter) *exec.Cmd {
	if command == "" {
		return nil
	}

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

	cmd := exec.Command(cmds[0], cmds[1:]...)
	cmd.Env = env
	cmd.Dir = dir
	if stdoutWriter != nil && stderrWriter != nil {
		cmd.Stdout = stdoutWriter
		cmd.Stderr = stderrWriter
	}

	return cmd
}
