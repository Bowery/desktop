// Copyright 2013-2014 Bowery, Inc.
// Contains the main entry point, service handling, and file watching.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"

	"github.com/Bowery/desktop/bowery/agent/plugin"
	"github.com/Bowery/gopackages/config"
	"github.com/gorilla/mux"
)

var (
	Env           = flag.String("env", "production", "If you want to run the agent in development mode uses different ports")
	InDevelopment = false
	Applications  = map[string]*Application{}
)

func main() {
	runtime.GOMAXPROCS(1)
	flag.Parse()
	if *Env == "development" {
		InDevelopment = true
	}

	// Register routes.
	router := mux.NewRouter()
	router.NotFoundHandler = NotFoundHandler
	for _, r := range Routes {
		route := router.NewRoute()
		route.Path(r.Path).Methods(r.Methods...)
		route.HandlerFunc(r.Handler)
	}

	port := config.BoweryAgentProdSyncPort
	if InDevelopment {
		port = config.BoweryAgentDevSyncPort
	}

	// Start the server.
	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: &SlashHandler{&LogHandler{os.Stdout, router}},
	}

	// Start tcp.
	go StartTCP()
	// Start event listening.
	pluginManager := plugin.SetPluginManager()
	go plugin.StartPluginListener()
	errStreamManager := NewErrStreamManager()
	go errStreamManager.PluginErrStream(pluginManager.Error)

	log.Println("Agent starting!")
	log.Fatal(server.ListenAndServe())
}
