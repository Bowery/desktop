// Copyright 2014 Bowery, Inc.
package main

import (
	"flag"

	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
)

var (
	env                = flag.String("env", "development", "Mode to run client in.")
	port               = flag.String("port", ":32055", "Port to listen on.")
	applicationManager *ApplicationManager
)

func main() {
	flag.Parse()

	router := mux.NewRouter()
	for _, r := range Routes {
		route := router.NewRoute()
		route.Path(r.Path).Methods(r.Method)
		route.HandlerFunc(r.Handler)
	}

	applicationManager = NewApplicationManager()

	app := negroni.Classic()
	app.UseHandler(&SlashHandler{router})
	app.Run(*port)
}
