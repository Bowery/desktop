// Copyright 2014 Bowery, Inc.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Bowery/delancey/delancey"
	"github.com/Bowery/gopackages/config"
	"github.com/Bowery/gopackages/schemas"
	"github.com/Bowery/gopackages/sys"
	"github.com/oguzbilgic/pusher"
)

// ContainerManager manages all active containers as well as
// the file syncing between the local and remote machines.
type ContainerManager struct {
	Containers map[string]*schemas.Container
	Syncer     *Syncer
}

// NewContainerManager creates a new ContainerManager.
func NewContainerManager() *ContainerManager {
	return &ContainerManager{
		Containers: make(map[string]*schemas.Container),
		Syncer:     NewSyncer(),
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
		ev := channel.Bind("update")
		data := (<-ev).(string)

		cont := new(schemas.Container)
		err = json.Unmarshal([]byte(data), cont)
		if err != nil {
			return
		}

		cont.LocalPath = container.LocalPath
		cm.Containers[container.ID] = cont
		cm.Syncer.Watch(cont)
		delancey.UploadSSH(cont, filepath.Join(os.Getenv(sys.HomeVar), ".ssh"))
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

	cm.Syncer.Remove(container)
	delete(cm.Containers, id)
	return nil
}

// Close closes the file syncer.
func (cm *ContainerManager) Close() error {
	return cm.Syncer.Close()
}
