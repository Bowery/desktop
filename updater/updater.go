// Copyright 2013-2014 Bowery, Inc.
package main

import (
	"bytes"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"bitbucket.org/kardianos/osext"
	"github.com/Bowery/gopackages/config"
	"github.com/Bowery/gopackages/rollbar"
	"github.com/Bowery/gopackages/sys"
	"github.com/Bowery/gopackages/update"
	"github.com/jeffchao/backoff"
)

const usage = `usage: updater [-d installDir] <update url> <current version> <command> [arguments]`

var (
	rollbarC      = rollbar.NewClient(config.RollbarToken, "production")
	pidSetter     = new(syncSetter)
	updatedSetter = new(syncSetter)
	restartSetter = &syncSetter{val: 1}
	updateURL     string
	version       string
	installDir    string
	err           error
)

func init() {
	flag.StringVar(&installDir, "d", "", "")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, usage)
	}
}

func main() {
	// Skip certificate errors on aws wildcard domains.
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	wait := false
	flag.Parse()
	args := flag.Args()

	if len(args) < 3 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	updateURL = args[0]
	version = args[1]
	cmdArgs := args[2:]

	// If version is empty try to detect it.
	if version == "" {
		cmd := exec.Command(cmdArgs[0], "--version")
		out, _ := cmd.CombinedOutput()
		version = strings.Trim(string(out), " \r\n")
	}
	if version == "" {
		err := errors.New("A version couldn't be detected, please provide a version")
		rollbarC.Report(err, nil)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	log.Println("Current version:", version)

	// Parse install directory.
	if installDir == "" || !filepath.IsAbs(installDir) {
		binDir, err := osext.ExecutableFolder()
		if err != nil {
			rollbarC.Report(err, map[string]string{
				"version": version,
			})
			log.Println(err)
			os.Exit(1)
		}

		installDir = filepath.Join(binDir, installDir)
	}

	// Check for updates.
	doUpdate() // Do update on start.
	go func() {
		for {
			<-time.After(1 * time.Hour)
			doUpdate()
		}
	}()

	// Setup the backoff limiter for restarts.
	exponential := backoff.Exponential()
	exponential.MaxRetries = 100

	// Continuously restart program.
	for restartSetter.Get() == 1 {
		if wait {
			if !exponential.Next() {
				err := killPid()
				if err != nil {
					rollbarC.Report(err, map[string]string{
						"version": version,
					})
					log.Println("Killing pid error:", err)
				}
				return
			}
			<-time.After(exponential.Delay)
		}
		wait = true
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin

		err := cmd.Start()
		if err != nil {
			rollbarC.Report(err, map[string]string{
				"version": version,
			})
			log.Println("Command error:", err)
			continue
		} else {
			pidSetter.Set(cmd.Process.Pid)
			log.Println("Starting process", cmdArgs, "with pid:", pidSetter.Get())
		}

		err = storePids()
		if err != nil {
			rollbarC.Report(err, map[string]string{
				"version": version,
			})
		}

		err = cmd.Wait()
		oldPid := pidSetter.Get()
		pidSetter.Set(0)
		if err != nil {
			log.Println("Command error:", err)

			// If the process was signaled don't wait.
			if cmd.ProcessState != nil {
				waitStatus, ok := cmd.ProcessState.Sys().(syscall.WaitStatus)
				if ok && waitStatus.Signaled() && updatedSetter.Get() == 1 {
					log.Println("Process with pid", oldPid, "was signaled to restart")
					wait = false
					continue
				}
			}

			rollbarC.Report(err, map[string]string{
				"version": version,
			})
			log.Println("Process with pid", oldPid, "failed to exit properly")
		} else {
			log.Println("Process with pid", oldPid, "has exited")
		}
	}

	// If we get here, we're killing the process from a signal.
	select {}
}

// syncSetter is a thread safe setter.
type syncSetter struct {
	val   int
	mutex sync.RWMutex
}

// Get a value thread safe.
func (ss *syncSetter) Get() int {
	ss.mutex.RLock()
	defer ss.mutex.RUnlock()

	return ss.val
}

// Set a value thread safe.
func (ss *syncSetter) Set(val int) {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	ss.val = val
}

func killPid() error {
	pid := pidSetter.Get()
	if pid > 0 {
		log.Println("Killing process tree for pid:", pid)
		proc, err := sys.GetPidTree(pid)
		if err != nil {
			return err
		}

		if proc != nil {
			err = proc.Kill()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// doUpdate will download any new version and replace binaries from the download.
func doUpdate() error {
	updatedSetter.Set(0)
	log.Println("Update is being checked")

	newVersion, newVersionURL, err := update.GetLatest(updateURL)
	if err != nil {
		rollbarC.Report(err, map[string]string{
			"version": version,
		})
		log.Println("Update error:", err)
		return err
	}

	changed, err := update.OutOfDate(version, newVersion)
	if err != nil {
		rollbarC.Report(err, map[string]string{
			"version":    version,
			"newVersion": newVersion,
		})
		log.Println("Update error:", err)
		return err
	}

	if !changed {
		log.Println("Version hasn't changed")
		return nil
	}

	log.Println("Getting contents for version", newVersion, "at", newVersionURL)
	contents, err := update.DownloadVersion(newVersionURL)
	if err != nil {
		rollbarC.Report(err, map[string]string{
			"version":    version,
			"newVersion": newVersion,
		})
		log.Println("Update error:", err)
		return err
	}

	log.Println("Replacing binaries found in downloaded version")
	var replaceErr error
	for info, body := range contents {
		path := filepath.Join(installDir, info.Name())

		if info.IsDir() {
			err := os.MkdirAll(path, info.Mode())
			if err != nil {
				replaceErr = err
				break
			}
			continue
		}

		err = sys.ReplaceBinPath(path, info, body)
		if err != nil {
			replaceErr = err
			break
		}
	}
	if replaceErr != nil {
		rollbarC.Report(err, map[string]string{
			"version":    version,
			"newVersion": newVersion,
		})
		log.Println("Update error:", replaceErr)
		return replaceErr
	}

	version = newVersion

	updatedSetter.Set(1)
	err = killPid()
	if err != nil {
		rollbarC.Report(err, map[string]string{
			"version": version,
		})
		log.Println("Update error:", err)
	}

	return err
}

// storePids saves the running pids in $TMPDIR/bowery_pids
func storePids() error {
	proc, err := sys.GetPidTree(os.Getpid())
	if err != nil {
		return err
	}
	pids := treeList(proc)

	file, err := os.Create(filepath.Join(os.TempDir(), "bowery_pids"))
	if err != nil {
		return err
	}
	defer file.Close()

	buf := bytes.NewBufferString(strings.Join(pids, "\n"))
	_, err = io.Copy(file, buf)
	return err
}

// treeList creates a list of a proc trees pids.
func treeList(proc *sys.Proc) []string {
	pids := []string{strconv.Itoa(proc.Pid)}

	if proc.Children != nil {
		for _, p := range proc.Children {
			pids = append(pids, treeList(p)...)
		}
	}

	return pids
}
