// Copyright Bowery, Inc. 2014
// This file contains schemas for the mercer server.
// It should eventually be migrated to gopackages once they're stable

package main

// Developer is the schema for a user in Bowery land.
type Developer struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Salt     string `json:"salt"`
}

// MetaFile is a munch of metadata around a file that mercer would sync
type MetaFile struct {
	LocalPath string `json:"localPath"`
	S3Path    string `json:"s3Path"`
	version   string `json:"version"`
	md5       string `json:"md5"`
}

// Env is an environment made of files belonging to a team
type Env struct {
	files []*MetaFile `json:"files"`
}

// Team is a group of developers who share a common environment.
type Team struct {
	ID      string       `json:"id"`
	Path    []string     `json:"path"`
	Members []*Developer `json:"members"`
	Master  *Env         `json:"environment"`
	Creator *Developer   `json:"creator"`
}
