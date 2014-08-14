// Copyright 2013-2014 Bowery, Inc.
// Contains the main entry point, service handling, and file watching.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
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

	err := os.MkdirAll(ServiceDir, os.ModePerm|os.ModeDir)
	if err == nil {
		err = os.Chdir(ServiceDir)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Register routes.
	router := mux.NewRouter()
	router.NotFoundHandler = NotFoundHandler
	for _, r := range Routes {
		route := router.NewRoute()
		route.Path(r.Path).Methods(r.Methods...)
		route.HandlerFunc(r.Handler)
	}

	port := config.BoweryAgentPort
	if InDevelopment {
		port = "3003"
	}

	// Start the server.
	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: &SlashHandler{&LogHandler{os.Stdout, router}},
	}

	// Start tcp.
	go StartTCP()

	// Start event listening.
	go plugin.StartPluginListener()

	// Set up debug http port, temporary.
	// todo(steve): remove once we know what the issue is.
	go func() {
		log.Println(http.ListenAndServe(":3004", nil))
	}()

	log.Println("Agent starting!")
	log.Fatal(server.ListenAndServe())
}
