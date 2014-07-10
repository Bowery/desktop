// Copyright 2014 Bowery, Inc.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
)

var (
	port       = flag.String("port", ":32055", "Port to run Bowery Desktop Client on.")
	isLoggedIn = false
)

func handler(rw http.ResponseWriter, req *http.Request) {
	fmt.Fprint(rw, "Hello World")
}

func main() {
	http.Handle("/", http.FileServer(http.Dir("./static/")))
	log.Fatal(http.ListenAndServe(*port, nil))
}
