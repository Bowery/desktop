// Copyright 2013-2014 Bowery, Inc.
package main

import (
	"encoding/json"

	"github.com/Bowery/desktop/bowery/agent/plugin"
)

type ErrStreamManager struct {
	tcp *TCP
}

func NewErrStreamManager() *ErrStreamManager {
	return &ErrStreamManager{
		tcp: NewTCP(),
	}
}

// PluginErrStream is meant to be ran in a go routine. Sends the error over esm's tcp
// connection to the client
func (esm *ErrStreamManager) PluginErrStream(pluginErrCh chan *plugin.PluginError) {
	for {
		esm.SendPluginError(<-pluginErrCh)
	}
}

// TODO(rm) figure this function out
// func (esm *ErrStreamManager) SendAppError(e error) (int, error) {
// 	msg, err := json.Marshal(map[string]string{
// 		"message": "",
// 	})
// 	if err != nil {
// 		return 1, err
// 	}
// 	return esm.tcp.Write(msg)
// }

// SendPluginErr marshals a json and writes it to the tcp connection
func (esm *ErrStreamManager) SendPluginError(pluginErr *plugin.PluginError) (int, error) {
	msg, err := json.Marshal(map[string]string{
		"pluginName": pluginErr.Plugin.Name,
		"message":    pluginErr.Error.Error(),
		"command":    pluginErr.Command,
	})
	if err != nil {
		return 1, err
	}
	return esm.tcp.Write(msg)
}
