// Copyright 2013-2014 Bowery, Inc.
// Contains the main entry point
package main

import (
	"fmt"
	"net/http"

	"github.com/Bowery/gopackages/schemas"
	"github.com/codegangsta/negroni"
	"github.com/unrolled/render"
)

var (
	AuthEndpoint   string = "broome.io"
	DaemonEndpoint string = "localhost:3000" // TODO (thebyrd) change this to match the toolbar app
	Me             schemas.Developer
)

type Application struct {
	ID         string // normalized as 80-maspeth-ave, but user enters 80 Maspeth Ave
	Name       string
	Start      string
	Build      string
	Env        map[string]string
	RemotePath string
	RemoteAddr string
	LocalPath  string
}

func getApps() []*Application {
	broome := &Application{
		ID:    "80-maspeth-ave",
		Name:  "Broome",
		Start: "./broome",
		Build: "make",
		Env: map[string]string{
			"ENV": "development",
		},
		RemotePath: "/home/bowery/broome",
		LocalPath:  "/Users/david/Documents/gocode/src/github.com/Bowery/broome/",
	}

	blog := &Application{
		ID:         "110-fifth-ave",
		Name:       "Blog",
		Start:      "node app.js",
		Build:      "npm install",
		RemotePath: "/home/david/blog",
		LocalPath:  "/Users/david/Documents/code/blog/",
	}

	return []*Application{broome, blog}
}

func main() {
	fmt.Println("Starting Client")
	r := render.New(render.Options{
		IndentJSON:    true,
		IsDevelopment: true, // TODO (thebyrd) remove in production
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		r.HTML(w, http.StatusOK, "home", map[string]interface{}{
			"Title":  "Home Page!",
			"Apps":   getApps(),
			"Status": "All Systems Go!",
		})
	})

	mux.HandleFunc("/applications/", func(w http.ResponseWriter, req *http.Request) {
		id := req.URL.Path[len("/applications/"):]
		var application *Application
		for _, app := range getApps() {
			if app.ID == id {
				application = app
				break
			}
		}
		r.HTML(w, http.StatusOK, "application", map[string]interface{}{
			"Application": application,
			"Status":      "Syncing...",
		})
	})

	app := negroni.Classic()
	app.UseHandler(mux)
	app.Run(":3001")
}
