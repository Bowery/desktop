package main

import (
	"fmt"
	"log"
	"net/http"
)

func handler(rw http.ResponseWriter, req *http.Request) {
	fmt.Fprint(rw, "Hello World")
}

func main() {
	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(":32055", nil))
}
