// Copyright 2014 Bowery, Inc.
package main

import (
	"encoding/json"
)

// appErr is basic error from the agent. Indicates app and the message
type appErr struct {
	AppID   string `json:"appID"`
	Message string `json:"message"`
}

// pluginErr stores information about plugin errors. Including the command that
// triggered them and what plugin was affected
type pluginErr struct {
	appErr
	PluginName string `json:"pluginName"`
	Command    string `json:"command"`
}

func ErrProcessor(s *Stream, data []byte) ([]byte, error) {
	perr := &pluginErr{}
	perr.AppID = s.application.ID
	json.Unmarshal(data, perr)
	return json.Marshal(map[string]interface{}{
		"appID":   s.application.ID,
		"message": string(data),
	})
}
