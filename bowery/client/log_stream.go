// Copyright 2014 Bowery, Inc.
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"
)

type syncWriter struct {
	File  *os.File
	mutex sync.Mutex
}

// Write writes the given buffer and syncs to the fs.
func (sw *syncWriter) Write(b []byte) (int, error) {
	sw.mutex.Lock()
	defer sw.mutex.Unlock()

	n, err := sw.File.Write(b)
	if err != nil {
		return n, err
	}

	return n, sw.File.Sync()
}

// Close closes the writer after any writes have completed.
func (sw *syncWriter) Close() error {
	sw.mutex.Lock()
	defer sw.mutex.Unlock()
	return sw.File.Close()
}

func LogProcessor(s *Stream, data []byte) ([]byte, error) {
	data = append(data, '\n')
	file, err := os.OpenFile(filepath.Join(logDir, s.application.ID+".log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	output := &syncWriter{File: file}
	buf := bytes.NewBuffer(data)
	if _, err := io.Copy(output, buf); err != nil {
		return nil, err
	}

	d := map[string]interface{}{}
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, err
	}

	return json.Marshal(map[string]interface{}{
		"appID":   s.application.ID,
		"message": d["message"],
	})
}
