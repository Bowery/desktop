// Copyright 2013-2014 Bowery, Inc.
// Contains routines to manage the service.
package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Bowery/desktop/bowery/agent/plugin"
	"github.com/Bowery/gopackages/sys"
)

var (
	ImageScriptsDir = "/image_scripts"
	mutex           sync.Mutex
	cmdStrs         = [4]string{} // Order: init, build, test start.
	// Only assign here if the process has been started.
	prevInitCmd  *exec.Cmd
	outputPath   = filepath.Join(os.Getenv(sys.HomeVar), ".bowery", "log")
	stdoutPath   = filepath.Join(outputPath, "stdout.log")
	stderrPath   = filepath.Join(outputPath, "stderr.log")
	stdoutWriter *OutputWriter
	stderrWriter *OutputWriter
)

func init() {
	stdoutWriter, _ = NewOutputWriter(stdoutPath)
	stderrWriter, _ = NewOutputWriter(stderrPath)
}

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

func Restart(initReset, reset bool, init, build, test, start string) chan bool {
	plugin.EmitPluginEvent(plugin.BEFORE_APP_RESTART, "", ServiceDir)
	mutex.Lock() // Lock here so no other restarts can interfere.
	finish := make(chan bool, 1)
	log.Println("Restarting")

	err := killCmds(initReset)
	if err != nil {
		mutex.Unlock()
		finish <- false
		return finish
	}

	// Create cmds.
	if reset {
		if !initReset {
			init = cmdStrs[0]
		}

		cmdStrs = [4]string{init, build, test, start}
	}

	initCmd := ParseCmd(cmdStrs[0], stdoutWriter, stderrWriter)
	buildCmd := ParseCmd(cmdStrs[1], stdoutWriter, stderrWriter)
	testCmd := ParseCmd(cmdStrs[2], stdoutWriter, stderrWriter)
	startCmd := ParseCmd(cmdStrs[3], stdoutWriter, stderrWriter)

	// Run in goroutine so commands can run in the background with the tcp
	// connection open.
	go func() {
		var wg sync.WaitGroup
		defer wg.Wait()
		defer mutex.Unlock()
		cmds := make([]*exec.Cmd, 0)

		// Get the image_scripts and start them.
		scriptPath := filepath.Join(ImageScriptsDir)
		dir, _ := os.Open(scriptPath)
		if dir != nil {
			infos, _ := dir.Readdir(0)
			if infos != nil {
				for _, info := range infos {
					if info.IsDir() {
						continue
					}

					cmd := ParseCmd(filepath.Join(scriptPath, info.Name()), stdoutWriter, stderrWriter)
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
			log.Println("Running Build Command")
			err := buildCmd.Run()
			if err != nil {
				stderrWriter.Write([]byte(err.Error() + "\n"))

				killCmds(initReset)
				finish <- false
				return
			}
			log.Println("Finished Build Command")
		}

		// Start the test command.
		if testCmd != nil {
			err := startProc(testCmd, stdoutWriter, stderrWriter)
			if err == nil {
				cmds = append(cmds, testCmd)
			}
		}

		// Start the init command if init is set.
		if initReset && initCmd != nil {
			err := startProc(initCmd, stdoutWriter, stderrWriter)
			if err == nil {
				prevInitCmd = initCmd
			}
		}

		// Start the start command.
		if startCmd != nil {
			err := startProc(startCmd, stdoutWriter, stderrWriter)
			if err == nil {
				cmds = append(cmds, startCmd)
			}
		}

		// Signal the start and prepare the wait group to keep tcp open.
		finish <- true
		wg.Add(len(cmds))

		// Wait for the init process to end
		if initReset && prevInitCmd != nil {
			wg.Add(1)

			go func(c *exec.Cmd) {
				waitProc(c, stdoutWriter, stderrWriter)
				wg.Done()
			}(prevInitCmd)
		}

		// Loop the commands and wait for them in parallel.
		for _, cmd := range cmds {
			go func(c *exec.Cmd) {
				waitProc(c, stdoutWriter, stderrWriter)
				wg.Done()
			}(cmd)
		}

		log.Println("Restart complete")
	}()

	plugin.EmitPluginEvent(plugin.AFTER_APP_RESTART, "", ServiceDir)
	return finish
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

// killCmds kills the running processes and resets them, the init cmd
// is only killed if init is true.
func killCmds(init bool) error {
	proc, err := GetPidTree(os.Getpid())
	if err != nil {
		return nil
	}

	for _, proc := range proc.Children {
		// Manage the previous init command properly.
		if prevInitCmd != nil && prevInitCmd.Process != nil &&
			prevInitCmd.Process.Pid == proc.Pid {
			if init {
				prevInitCmd = nil
			} else {
				continue
			}
		}

		// Don't kill any plugin background processes.
		isPluginBackgroundProcess := false
		for _, p := range plugin.GetPlugins() {
			log.Println(p)
			if p.BackgroundCommand != nil {
				if p.BackgroundCommand.Process.Pid == proc.Pid {
					isPluginBackgroundProcess = true
					break
				}
			}
		}

		if isPluginBackgroundProcess {
			continue
		}

		err = proc.Kill()
		if err != nil {
			return err
		}
	}

	return nil
}

// ParseCmd converts a string to a command, connecting stdio to a
// tcp connection.
func ParseCmd(command string, stdoutWriter, stderrWriter *OutputWriter) *exec.Cmd {
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
	if stdoutWriter != nil && stderrWriter != nil {
		cmd.Stdout = stdoutWriter
		cmd.Stderr = stderrWriter
	}

	return cmd
}
