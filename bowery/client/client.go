// Copyright 2014 Bowery, Inc.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
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
	AbsPath            string
	VERSION            string // This is set when release_client.sh is ran.
)

func main() {
	ver := false
	flag.StringVar(&env, "env", "development", "Mode to run client in.")
	flag.StringVar(&port, "port", ":32055", "Port to listen on.")
	flag.BoolVar(&ver, "version", false, "Print the version")
	flag.Parse()
	if ver {
		fmt.Println(VERSION)
		return
	}

	rollbarC = rollbar.NewClient(config.RollbarToken, env)
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

	abs, _ := filepath.Abs(filepath.Join(filepath.Dir(os.Args[0]), "../ui/"))
	AbsPath = abs

	router := mux.NewRouter()
	router.NotFoundHandler = http.FileServer(http.Dir(abs))
	for _, r := range Routes {
		route := router.NewRoute()
		route.Path(r.Path).Methods(r.Method)
		route.HandlerFunc(r.Handler)
	}

	go ssePool.run()

	app := negroni.Classic()
	app.UseHandler(&SlashHandler{router})
	app.Run(port)
}
