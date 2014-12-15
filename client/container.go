// Copyright 2014 Bowery, Inc.

package main

import (
	"fmt"

	"github.com/Bowery/gopackages/schemas"
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
	cm.Syncer.Watch(container)
	cm.Containers[container.ID] = container
}

// RemoveByID removes a container with the specified id and
// ends the associated file watching.
func (cm *ContainerManager) RemoveByID(id string) error {
	container, ok := cm.Containers[id]
	if !ok {
		return fmt.Errorf("no container with id %s exists", id)
	}

	err := cm.Syncer.Remove(container)
	if err != nil {
		return err
	}

	delete(cm.Containers, id)
	return nil
}

// Close closes the file syncer.
func (cm *ContainerManager) Close() error {
	return cm.Syncer.Close()
}
