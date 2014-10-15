// Copyright 2013-2014 Bowery, Inc.
package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/tls"
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
	"github.com/Bowery/gopackages/config"
	"github.com/Bowery/gopackages/keen"
	"github.com/Bowery/gopackages/rollbar"
	"github.com/Bowery/gopackages/sys"
	goversion "github.com/hashicorp/go-version"
)

const usage = `usage: updater <update url> <current version> <install dir> <command> [arguments]`

var (
	ErrNotFound = errors.New("Update version url not found for current system")
	rollbarC    = rollbar.NewClient(config.RollbarToken, "production")
	keenC       = &keen.Client{
		WriteKey:  config.KeenWriteKey,
		ProjectID: config.KeenProjectID,
	}
	pid        = 0
	mutex      sync.RWMutex
	updateURL  string
	version    string
	installDir string
	err        error
)

func main() {
	// Skip certificate errors on aws wildcard domains.
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	wait := false

	if len(os.Args) < 5 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	updateURL = os.Args[1]
	version = os.Args[2]
	installDir = os.Args[3]
	cmdArgs := os.Args[4:]

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
			rollbarC.Report(err, map[string]string{
				"version": version,
			})
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

			rollbarC.Report(err, map[string]string{
				"version": version,
			})
			log.Println("Process with pid", oldPid, "failed to exit properly")
		} else {
			log.Println("Process with pid", oldPid, "has exited")
		}
	}
}

// setPid sets the current pid.
func setPid(val int) {
	mutex.Lock()
	defer mutex.Unlock()

	pid = val
}

// getPid retrieves the current pid.
func getPid() int {
	mutex.RLock()
	defer mutex.RUnlock()

	return pid
}

// doUpdate will download any new version and replace binaries from the download.
func doUpdate() error {
	log.Println("Update is being checked")

	newVersion, newVersionURL, err := getVersion(updateURL)
	if err != nil {
		rollbarC.Report(err, map[string]string{
			"version": version,
		})
		log.Println("Update error:", err)
		return err
	}

	var newV *goversion.Version
	oldV, err := goversion.NewVersion(version)
	if err == nil {
		newV, err = goversion.NewVersion(newVersion)
	}
	if err != nil {
		rollbarC.Report(err, map[string]string{
			"version":    version,
			"newVersion": newVersion,
		})
		log.Println("Update error:", err)
		return err
	}

	if oldV.Equal(newV) || oldV.GreaterThan(newV) {
		log.Println("Version hasn't changed")
		return nil
	}

	log.Println("Getting contents for version", newVersion, "at", newVersionURL)
	contents, err := getVersionDownload(newVersionURL)
	if err != nil {
		rollbarC.Report(err, map[string]string{
			"version":    version,
			"newVersion": newVersion,
		})
		log.Println("Update error:", err)
		return err
	}

	log.Println("Replacing binaries found in downloaded version")
	var replaceErr error = nil
	for header, body := range contents {
		info := header.FileInfo()
		path := filepath.Join(installDir, filepath.Join(strings.Split(header.Name, "/")...))

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

	keenC.AddEvent("agent update", map[string]string{
		"oldVersion": version,
		"newVersion": newVersion,
	})
	version = newVersion

	pid := getPid()
	if pid > 0 {
		log.Println("Killing process tree for pid:", pid)
		proc, err := sys.GetPidTree(pid)
		if err != nil {
			rollbarC.Report(err, map[string]string{
				"version": version,
			})
			log.Println("Update error:", err)
			return err
		}

		if proc != nil {
			err = proc.Kill()
			if err != nil {
				rollbarC.Report(err, map[string]string{
					"version": version,
				})
				log.Println("Update error:", err)
			}
		}
	}

	return nil
}

// getVersion gets the most recent version url from an update url
func getVersion(url string) (string, string, error) {
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

// getVersionDownload retrieves and untars the versions archive into a file map.
func getVersionDownload(url string) (map[*tar.Header]io.Reader, error) {
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode < http.StatusOK || res.StatusCode >= 300 {
		return nil, errors.New("Status code not in 2xx class: " + res.Status)
	}
	contents := make(map[*tar.Header]io.Reader)

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

		contents[hdr] = buf
	}

	return contents, nil
}
