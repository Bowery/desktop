// Copyright 2013-2014 Bowery, Inc.
// Contains the main entry point, service handling, and file watching.
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"

	"github.com/gorilla/mux"
)

func main() {
	runtime.GOMAXPROCS(1)

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

	port := os.Getenv("PORT")
	if port == "" {
		port = "3001"
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

	log.Println("Agent starting!")
	log.Fatal(server.ListenAndServe())
}
