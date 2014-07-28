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
	"time"

	"github.com/gorilla/mux"
)

var (
	Env           = flag.String("env", "production", "If you want to run the agent in development mode uses different ports")
	InDevelopment = false
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

	port := "3001"
	if InDevelopment {
		port = "3003"
	}

	// Start the server.
	server := &http.Server{
		Addr:    ":" + port,
		Handler: &SlashHandler{&LogHandler{os.Stdout, router}},
	}

	// Start tcp.
	go StartTCP()

	// Start event listening.
	go StartPluginListener()

	go func() {
		<-time.After(2 * time.Second)
		pluginManager.Event <- &Event{
			Type: "after-update",
		}
	}()

	log.Println("Agent starting!")
	log.Fatal(server.ListenAndServe())
}
