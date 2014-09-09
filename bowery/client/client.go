// Copyright 2014 Bowery, Inc.
package main

import (
	"flag"
	"net/http"
	"path/filepath"

	"github.com/Bowery/gopackages/config"
	"github.com/Bowery/gopackages/rollbar"
	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
)

var (
	env                string
	port               string
	applicationManager *ApplicationManager
	rollbarC           *rollbar.Client
)

func main() {
	flag.StringVar(&env, "env", "development", "Mode to run client in.")
	flag.StringVar(&port, "port", ":32055", "Port to listen on.")
	flag.Parse()

	rollbarC = rollbar.NewClient(config.RollbarToken, env)
	applicationManager = NewApplicationManager()
	defer applicationManager.Close()

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
