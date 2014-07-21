// Copyright 2013-2014 Bowery, Inc.
package main

import (
	"log"
	"net/http"
	"path/filepath"

	"github.com/Bowery/gopackages/localdb"
	"github.com/Bowery/gopackages/schemas"
	"github.com/Bowery/gopackages/sys"
	"github.com/codegangsta/negroni"
	"github.com/unrolled/render"
)

var (
	AuthEndpoint   string = "broome.io"
	DaemonEndpoint string = "localhost:3000" // TODO (thebyrd) change this to match the toolbar app
	db             *localdb.DB
	data           *localData
	r              = render.New(render.Options{
		IndentJSON:    true,
		IsDevelopment: true,
	})
)

type localData struct {
	Developer    *schemas.Developer
	Applications []*schemas.Application
}

// Set up local db.
func init() {
	var err error
	db, err = localdb.New(filepath.Join(sys.HomeVar, ".bowery_state"))
	if err != nil {
		log.Println("Unable to create local database.")
		return
	}

	// data = new(localData)
	// if err := db.Load(data)

	// db.Load()
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", indexHandler)

	app := negroni.Classic()
	app.UseHandler(mux)
	app.Run(":3001")
}

func indexHandler(rw http.ResponseWriter, req *http.Request) {
	r.HTML(rw, http.StatusOK, "home", map[string]interface{}{
		"Title":  "Home Page!",
		"Status": "All Systems Go!",
	})
}
