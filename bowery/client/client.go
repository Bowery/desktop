// Copyright 2014 Bowery, Inc.
package main

import (
	"flag"
	"log"
	"net/http"
	"path/filepath"

	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
)

var (
	env                string
	port               string
	applicationManager *ApplicationManager
)

func main() {
	flag.StringVar(&env, "env", "development", "Mode to run client in.")
	flag.StringVar(&port, "port", ":32055", "Port to listen on.")
	flag.Parse()

	applicationManager = NewApplicationManager()
	defer applicationManager.Close()

	go func() {
		for {
			select {
			case ev := <-applicationManager.Syncer.Event:
				log.Println(ev)
			case err := <-applicationManager.Syncer.Error:
				log.Println(err)
			}
		}
	}()

	abs, _ := filepath.Abs("ui/")

	router := mux.NewRouter()
	router.NotFoundHandler = http.FileServer(http.Dir(abs))
	for _, r := range Routes {
		route := router.NewRoute()
		route.Path(r.Path).Methods(r.Method)
		route.HandlerFunc(r.Handler)
	}

	app := negroni.Classic()
	app.UseHandler(&SlashHandler{router})
	app.Run(port)
}
