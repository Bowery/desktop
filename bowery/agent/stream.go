// Copyright 2013-2014 Bowery, Inc.
package main

import (
	"encoding/json"

	"github.com/Bowery/desktop/bowery/agent/plugin"
)

// StreamManager keeps track of the connection with the client as well as the errors
// coming from plugins
type StreamManager struct {
	tcp       *TCP
	pluginErr chan *plugin.PluginError
	// there should also be two more channels here that contain applicatoin errors
	// and logs so that the `Stream` method can more uniformly handle everything that
	// is to be sent to the tcp connection with the client
}

// NewStreamManager returns a stream manager with an instantiated TCP connection
// BUG: when a client disconnects, it can't reconnect w/o restarting the server
func NewStreamManager() *StreamManager {
	return &StreamManager{
		tcp: NewTCP(),
	}
}

// Stream listens on a StreamManager's channels and passes along the data to the
// appropriate handlers for processing the data and sending it to the client
func (sm *StreamManager) Stream() {
	for {
		select {
		case pe := <-sm.pluginErr:
			sm.SendPluginError(pe)
		}
		// once stream manager has channels for logs and application errors, the
		// logic for sending those out should be moved into here as well
	}
}

// SendPluginErr marshals a json and writes it to the tcp connection
func (sm *StreamManager) SendPluginError(pluginErr *plugin.PluginError) (int, error) {
	msg, err := json.Marshal(map[string]string{
		"type":       "plugin_error",
		"pluginName": pluginErr.Plugin.Name,
		"message":    pluginErr.Error.Error(),
		"command":    pluginErr.Command,
	})
	if err != nil {
		return 0, err
	}
	return sm.tcp.Write(msg)
}

// SendLog generates a json object that indicates that it contains a log (with its acompanying message).
// The resulting json is then written to the tcp connection
func (sm *StreamManager) SendLog(data []byte) (int, error) {
	msg, err := json.Marshal(map[string]string{
		"type":    "log",
		"message": string(data),
	})
	if err != nil {
		return 0, err
	}
	return sm.tcp.Write(msg)
}

// there should also be a send application error method that is called in `Stream`
