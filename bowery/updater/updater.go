// Copyright 2013-2014 Bowery, Inc.
package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"bitbucket.org/kardianos/osext"
	"github.com/Bowery/gopackages/sys"
)

const usage = `usage: updater <update url> <current version> <command> [arguments]`

var (
	ErrNotFound = errors.New("Update version url not found for current system")
)

func main() {
	var mutex sync.RWMutex
	wait := false
	pid := 0
	setPid := func(val int) {
		mutex.Lock()
		defer mutex.Unlock()

		pid = val
	}
	getPid := func() int {
		mutex.RLock()
		defer mutex.RUnlock()

		return pid
	}

	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	updateURL := os.Args[1]
	version := os.Args[2]
	cmdArgs := os.Args[3:]

	// If version is empty try to detect it.
	if version == "" {
		cmd := exec.Command(cmdArgs[0], "--version")
		out, _ := cmd.CombinedOutput()
		version = strings.Trim(string(out), " \r\n")
	}
	if version == "" {
		fmt.Fprintln(os.Stderr, "A version couldn't be detected please provide a version")
		os.Exit(1)
	}
	log.Println("Current version:", version)

	binDir, err := osext.ExecutableFolder()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	// Check for updates.
	go func() {
		for {
			<-time.After(4 * time.Hour)
			log.Println("Update is being checked")

			newVersion, newVersionURL, err := checkUpdate(updateURL)
			if err != nil {
				log.Println("Update error:", err)
				continue
			}

			if newVersion == version {
				log.Println("Version hasn't changed")
				continue
			}

			log.Println("Getting contents for version", newVersion, "at", newVersionURL)
			contents, err := getVersion(newVersionURL)
			if err != nil {
				log.Println("Update error:", err)
				continue
			}

			log.Println("Replacing binaries found in downloaded version")
			var replaceErr error = nil
			for info, body := range contents {
				if !isExecutable(info) {
					continue
				}

				// Find existing path.
				path, err := exec.LookPath(info.Name())
				if err == nil {
					err = sys.ReplaceBinPath(path, body)
					if err != nil {
						replaceErr = err
						break
					}
					continue
				}

				// Doesn't exist or existing isn't exec.
				err = sys.ReplaceBinPath(filepath.Join(binDir, info.Name()), body)
				if err != nil {
					replaceErr = err
					break
				}
			}
			if replaceErr != nil {
				log.Println("Update error:", replaceErr)
				continue
			}

			version = newVersion
			pid := getPid()
			if pid > 0 {
				log.Println("Killing process tree for pid:", pid)
				proc, err := sys.GetPidTree(pid)
				if err != nil {
					log.Println("Update error:", err)
					continue
				}

				if proc != nil {
					err = proc.Kill()
					if err != nil {
						log.Println("Update error:", err)
					}
				}
			}
		}
	}()

	// Continuously restart program.
	for {
		if wait {
			<-time.After(5 * time.Second)
		}
		wait = true // Wait by default after the first loop.
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Start()
		if err != nil {
			log.Println("Command error:", err)
			continue
		} else {
			setPid(cmd.Process.Pid)
			log.Println("Starting process", cmdArgs, "with pid:", getPid())
		}

		err = cmd.Wait()
		oldPid := getPid()
		setPid(0)
		if err != nil {
			log.Println("Command error:", err)

			// If the process was signaled don't wait.
			if cmd.ProcessState != nil {
				waitStatus, ok := cmd.ProcessState.Sys().(syscall.WaitStatus)
				if ok && waitStatus.Signaled() {
					log.Println("Process with pid", oldPid, "was signaled to restart")
					wait = false
					continue
				}
			}

			log.Println("Process with pid", oldPid, "failed to exit properly")
		} else {
			log.Println("Process with pid", oldPid, "has exited")
		}
	}
}

// checkUpdate gets the most recent version url from an update url
func checkUpdate(url string) (string, string, error) {
	res, err := http.Get(url)
	if err != nil {
		return "", "", err
	}
	defer res.Body.Close()

	if res.StatusCode < http.StatusOK || res.StatusCode >= 300 {
		return "", "", errors.New("Status code not in 2xx class: " + res.Status)
	}

	// Scan each line looking for the version url for the current system.
	version := ""
	line := 0
	scanner := bufio.NewScanner(res.Body)
	for scanner.Scan() {
		text := scanner.Text()
		line++
		if line <= 1 {
			version = text
			continue
		}

		if strings.Contains(text, runtime.GOOS) && strings.Contains(text, runtime.GOARCH) {
			return version, text, nil
		}
	}

	err = scanner.Err()
	if err != nil {
		return "", "", err
	}

	return "", "", ErrNotFound
}

// getVersion retrieves and untars the versions archive into a file map.
func getVersion(url string) (map[os.FileInfo]io.Reader, error) {
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode < http.StatusOK || res.StatusCode >= 300 {
		return nil, errors.New("Status code not in 2xx class: " + res.Status)
	}
	contents := make(map[os.FileInfo]io.Reader)

	body, err := gzip.NewReader(res.Body)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	archive := tar.NewReader(body)

	for {
		hdr, err := archive.Next()
		if err != nil && err != io.EOF {
			return nil, err
		}
		if hdr == nil || err == io.EOF {
			break
		}

		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, archive)
		if err != nil {
			return nil, err
		}

		contents[hdr.FileInfo()] = buf
	}

	return contents, nil
}

// isExecutable checks if a os.FileInfo describes an executable file.
func isExecutable(info os.FileInfo) bool {
	if info.IsDir() {
		return false
	}

	// Generic mode check.
	if info.Mode()&0111 != 0 {
		return true
	}

	// Windows style with PATHEXT.
	ext := strings.ToLower(filepath.Ext(info.Name()))
	list := filepath.SplitList(os.Getenv("PATHEXT"))
	for _, supportedExt := range list {
		if ext == strings.ToLower(supportedExt) {
			return true
		}
	}

	return false
}
