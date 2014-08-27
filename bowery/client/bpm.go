// Copyright 2013-2014 Bowery, Inc.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Bowery/gopackages/schemas"
	"github.com/Bowery/gopackages/sys"
	"github.com/Bowery/gopackages/util"
)

var (
	boweryDir  = filepath.Join(os.Getenv(sys.HomeVar), ".bowery")
	formulaDir = filepath.Join(boweryDir, "formulae")
	PluginDir  = filepath.Join(boweryDir, "plugins")
	repoName   = "plugins"
	gitHub     = "https://github.com/"
	formulae   map[string]schemas.Formula // more efficient than iterating through a slice
)

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

// processFormulae, reads all the json files and makes the appropriate data structure
func processFormulae(isDev bool) error {
	files, err := ioutil.ReadDir(formulaDir)
	if err != nil {
		return err
	}

	formulae = map[string]schemas.Formula{}
	devFormulae := map[string]schemas.Formula{}
	for _, fileInfo := range files {
		if strings.Contains(fileInfo.Name(), ".json") {
			var err error
			file, err := ioutil.ReadFile(filepath.Join(formulaDir, fileInfo.Name()))
			if err != nil {
				log.Printf("%v cannot be opened", fileInfo.Name())
				continue
			}

			var formula *schemas.Formula
			if formula, err = util.ValidateFormula(file); err != nil {
				log.Printf("%v is not valid. Error: %s", fileInfo.Name(), err.Error())
				continue
			}

			if isDev && strings.Contains(fileInfo.Name(), ".dev") {
				devFormulae[formula.Name] = *formula
			} else {
				formulae[formula.Name] = *formula
			}
		}
	}

	// overwrite formula where dev formula exist
	for name, formula := range devFormulae {
		formulae[name] = formula
	}

	return nil
}

// CreateFormulae creates a new dev formula.
func CreateFormulae(name, desc, repo string, osList, depsList []string, dev *schemas.Developer) error {
	formulae := schemas.Formula{
		Name:        name,
		Description: desc,
		Requirements: schemas.Requirements{
			OS:   osList,
			Deps: depsList,
		},
		Author: schemas.Author{
			Name:  dev.Name,
			Email: dev.Email,
		},
		Hooks: schemas.Hooks{
			OnPluginInit: "echo 'Put Your Hook Here'",
		},
		Version:    "1.0.0",
		Repository: repo,
	}

	file, err := os.OpenFile(filepath.Join(formulaDir, fmt.Sprintf("%s.dev.json", name)),
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	data, err := json.MarshalIndent(formulae, "", "  ")
	if err != nil {
		return err
	}
	buf := bytes.NewBuffer(data)
	_, err = io.Copy(file, buf)

	if err != nil {
		return err
	}

	return processFormulae(true)
}

// func watchDevFormula(name string) error {
// 	formula, ok := GetFormulaByName(name)
// 	if !ok {
// 		return errors.New("invalid plugin name")
// 	}

// 	return nil
// }

// UpdateFormulae, checks to see if there's a directory for the formulae already.
// If there is, it `git pull`s it. Otherwise, it `git clone`s the repo.
func UpdateFormulae(isDev bool) error {
	os.Chdir(boweryDir)
	defer os.Chdir(TemplateDir)

	if _, err := os.Stat(formulaDir); err == nil {
		os.Chdir(formulaDir)

		if err := git("pull"); err != nil {
			return err
		}

		return processFormulae(isDev)
	}

	if err := git("clone", gitHub+"Bowery/"+repoName+".git", "formulae"); err != nil {
		return err
	}

	return processFormulae(isDev)
}

// GetFormulae, returns a slice of all the schemas.Formula. Even though, internally, formulae
// is a map (for effeciency), it is returned as a slice to the caller
func GetFormulae() []schemas.Formula {
	results := make([]schemas.Formula, len(formulae))
	i := 0
	for _, formula := range formulae {
		results[i] = formula
		i += 1
	}
	return results
}

// GetFormulaByName, given an input string, it returns a formula with that name
func GetFormulaByName(name string) (schemas.Formula, bool) {
	i, ok := formulae[name]
	return i, ok
}

// SearchFormulae, variadic function that takes any number of search terms. Is a
// very, very naive search where any plugin where the name, description, or
// dependancies contains a search term makes the plugin a result
func SearchFormulae(terms ...string) ([]schemas.Formula, error) {
	results := []schemas.Formula{}
	for _, formula := range formulae {
		formTerms := []string{formula.Name, formula.Description}
		if formula.Requirements.Deps != nil {
			formTerms = append(formTerms, formula.Requirements.Deps...)
		}

		for _, term := range terms {
			if strings.Contains(strings.Join(formTerms, " "), term) {
				results = append(results, formula)
				break
			}
		}
	}

	return results, nil
}

// InstallPlugin, given the name of a plugin, it installs the latest version. TODO:
// allow for specific versions to be installable, also send to agent
func InstallPlugin(name string) (string, error) {
	formula, ok := GetFormulaByName(name)
	if !ok {
		return "", errors.New(fmt.Sprintf("No formula by name `%s`.", name))
	}

	os.Chdir(PluginDir)
	defer os.Chdir(TemplateDir)

	// Determine if the repository is hosted or on the local machine.
	u, err := url.Parse(formula.Repository)
	// Is git repo.
	if err == nil && u.Host != "" {
		dirName := fmt.Sprintf("%s@%s", formula.Name, formula.Version)
		if _, err := os.Stat(dirName); err == nil {
			return filepath.Join(PluginDir, dirName), nil
		}

		if err := git("clone", formula.Repository, dirName); err != nil {
			return "", err
		}

		// cd into repo
		if err := os.Chdir(dirName); err != nil {
			return "", err
		}

		// if a commit is specified, checkout.
		if formula.Commit != "" {
			if err := git("checkout", formula.Commit); err != nil {
				return "", err
			}
		}

		// remove the .git directory
		if err := os.RemoveAll(".git"); err != nil {
			return "", err
		}

		// cd a level up
		if err := os.Chdir(PluginDir); err != nil {
			return "", err
		}

		return filepath.Join(PluginDir, dirName), nil
		// Is on local machine
	} else if err == nil && u.Host == "" {
		return formula.Repository, nil
	} else if err != nil {
		return "", err
	}

	return "", err
}
