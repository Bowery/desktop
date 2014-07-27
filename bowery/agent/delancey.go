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

	// Start the server.
	server := &http.Server{
		Addr:    ":3001",
		Handler: &SlashHandler{&LogHandler{os.Stdout, router}},
	}

	// Start tcp.
	go StartTCP()

	log.Println("Delancey starting!")
	log.Fatal(server.ListenAndServe())
}
