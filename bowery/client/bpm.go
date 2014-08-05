// Copyright 2013-2014 Bowery, Inc.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Bowery/gopackages/sys"
)

var (
	boweryDir  = filepath.Join(os.Getenv(sys.HomeVar), ".bowery")
	formulaDir = filepath.Join(boweryDir, "formulae")
	PluginDir  = filepath.Join(boweryDir, "plugins")
	repoName   = "plugins"
	gitHub     = "https://github.com/"
	formulae   map[string]Formula // more efficient than iterating through a slice
)

type Author struct {
	Name    string `json:"name"`
	GitHub  string `json:"github,omitempty"`
	Email   string `json:"email,omitempty"`
	Twitter string `json:"twitter,omitempty"`
}

type Hooks struct {
	OnPluginStart    string `json:"on-plugin-start,omitempty"`
	BeforeAppRestart string `json:"before-app-restart,omitempty"`
	AfterAppRestart  string `json:"after-app-restart,omitempty"`
	BeforeAppUpdate  string `json:"before-app-update,omitempty"`
	AfterAppUpdate   string `json:"after-app-update,omitempty"`
	BeforeAppDelete  string `json:"before-app-delete,omitempty"`
	AfterAppDelete   string `json:"after-app-delete,omitempty"`
	BeforeFileUpdate string `json:"before-file-update,omitempty"`
	AfterFileUpdate  string `json:"after-file-update,omitempty"`
	BeforeFileCreate string `json:"before-file-create,omitempty"`
	AfterFileCreate  string `json:"after-file-create,omitempty"`
	BeforeFileDelete string `json:"before-file-delete,omitempty"`
	AfterFileDelete  string `json:"after-file-delete,omitempty"`
	BeforeFullUpload string `json:"before-full-upload,omitempty"`
	AfterFullUpload  string `json:"after-full-upload,omitempty"`
}

type Formula struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Deps        string `json:"deps,omitempty"`
	Author      `json:"author"`
	Hooks       `json:"hooks"`
	Repository  string `json:"repository"`
	Version     string `json:"version"`
}

// git, glorified exec. The error returned is Stderr
func git(args ...string) error {
	cmd := exec.Command("git", args...)
	var stdErr bytes.Buffer
	cmd.Stderr = &stdErr

	if err := cmd.Run(); err != nil {
		return errors.New(strings.TrimSpace(stdErr.String()))
	}

	return nil
}

// ProcessFormulae, reads all the json files and makes the appropriate data structure
func processFormulae() error {
	files, err := ioutil.ReadDir(formulaDir)
	if err != nil {
		return err
	}

	formulae = map[string]Formula{}
	for _, fileInfo := range files {
		if strings.Contains(fileInfo.Name(), ".json") {
			file, err := ioutil.ReadFile(filepath.Join(formulaDir, fileInfo.Name()))
			if err != nil {
				log.Printf("%v cannot be opened", fileInfo.Name())
				continue
			}

			var formula Formula
			if err := json.Unmarshal(file, &formula); err != nil {
				log.Printf("%v cannot be parsed", fileInfo.Name())
				continue
			}
			formulae[formula.Name] = formula
		}
	}

	return nil
}

// UpdateFormulae, checks to see if there's a directory for the formulae already.
// If there is, it `git pull`s it. Otherwise, it `git clone`s the repo.
func UpdateFormulae() error {
	os.Chdir(boweryDir)
	defer os.Chdir(TemplateDir)

	if _, err := os.Stat(formulaDir); err == nil {
		os.Chdir(formulaDir)

		if err := git("pull"); err != nil {
			return err
		}

		return processFormulae()
	}

	if err := git("clone", gitHub+"Bowery/"+repoName+".git", "formulae"); err != nil {
		return err
	}

	return processFormulae()
}

// GetFormulae, returns a slice of all the Formula. Even though, internally, formulae
// is a map (for effeciency), it is returned as a slice to the caller
func GetFormulae() []Formula {
	results := make([]Formula, len(formulae))
	i := 0
	for _, formula := range formulae {
		results[i] = formula
		i += 1
	}
	return results
}

// GetFormulaByName, given an input string, it returns a formula with that name
func GetFormulaByName(name string) (Formula, bool) {
	i, ok := formulae[name]
	return i, ok
}

// SearchFormulae, variadic function that takes any number of search terms. Is a
// very, very naive search where any plugin where the name, description, or
// dependancies contains a search term makes the plugin a result
func SearchFormulae(terms ...string) ([]Formula, error) {
	results := []Formula{}
	for _, formula := range formulae {
		for _, term := range terms {
			if strings.Contains(strings.Join([]string{
				formula.Name, formula.Description, formula.Deps,
			}, " "), term) {
				results = append(results, formula)
				break
			}
		}
	}

	return results, nil
}

// InstallPlugin, given the name of a plugin, it installs the latest version. TODO:
// allow for specific versions to be installable, also send to agent
func InstallPlugin(name string) error {
	formula, ok := GetFormulaByName(name)
	if !ok {
		return errors.New(fmt.Sprintf("No formula by name `%s`.", name))
	}

	os.Chdir(PluginDir)
	defer os.Chdir(TemplateDir)

	ver := strings.Split(formula.Version, "@")
	version := ver[0]
	commit := ver[1]
	dirName := formula.Name + "@" + version

	if _, err := os.Stat(formula.Name + "@" + version); err == nil {
		return nil
	}

	if err := git("clone", formula.Repository, dirName); err != nil {
		return err
	}

	// cd into repo
	if err := os.Chdir(dirName); err != nil {
		return err
	}

	// checkout the right version
	if err := git("checkout", commit); err != nil {
		return err
	}

	// remove the .git directory
	if err := os.RemoveAll(".git"); err != nil {
		return err
	}

	// cd a level up
	if err := os.Chdir(PluginDir); err != nil {
		return err
	}

	return nil
}
