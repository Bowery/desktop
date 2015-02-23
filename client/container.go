// Copyright 2014 Bowery, Inc.

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Bowery/delancey/delancey"
	"github.com/Bowery/gopackages/config"
	"github.com/Bowery/gopackages/schemas"
	"github.com/Bowery/gopackages/sys"
	"github.com/Bowery/pusher"
)

// ContainerManager manages all active containers as well as
// the file syncing between the local and remote machines.
type ContainerManager struct {
	Containers map[string]*schemas.Container
}

// NewContainerManager creates a new ContainerManager.
func NewContainerManager() *ContainerManager {
	return &ContainerManager{
		Containers: make(map[string]*schemas.Container),
	}
}

// Add adds a container and initiates file syncing.
func (cm *ContainerManager) Add(container *schemas.Container) {
	go func() {
		conn, err := pusher.New(config.PusherKey)
		if err != nil {
			return
		}
		channel := conn.Channel("container-" + container.ID)
		ev := channel.Bind("created")
		data := (<-ev).(string)
		conn.Disconnect()

		cont := new(schemas.Container)
		err = json.Unmarshal([]byte(data), cont)
		if err != nil {
			return
		}

		cont.LocalPath = container.LocalPath
		cm.Containers[container.ID] = cont
		/*
			cm.Syncer.Watch(cont)
		*/
		delancey.UploadSSH(cont, filepath.Join(os.Getenv(sys.HomeVar), ".ssh"))

    fmt.Println("created, new container", cont)
		fmt.Println("error!", runCommand("mount", "-t", "nfs4", "-o", "proto=tcp,port=2049",
			cont.Address+":"+cont.RemotePath, cont.LocalPath))
	}()

	cm.Containers[container.ID] = container
}

// RemoveByID removes a container with the specified id and
// ends the associated file watching.
func (cm *ContainerManager) RemoveByID(id string) error {
	container, ok := cm.Containers[id]
	if !ok {
		return fmt.Errorf("no container with id %s exists", id)
	}

	err := runCommand("umount", container.LocalPath)
	if err != nil {
		return nil
	}

	/*
		cm.Syncer.Remove(container)
	*/
	delete(cm.Containers, id)
	return nil
}

// Close closes the file syncer.
func (cm *ContainerManager) Close() error {
	return nil
}

// runCommand will run the given command capturing stderr as the error if the
// command fails.
func runCommand(name string, args ...string) error {
	fmt.Println("running", name, args)
	var stderr bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if !cmd.ProcessState.Success() {
			msg := strings.TrimRight(stderr.String(), "\r\n")
			msg = strings.TrimSpace(msg)

			return errors.New(msg)
		}

		return err
	}

	return nil
}
