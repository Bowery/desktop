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
	"github.com/Bowery/gopackages/util"
	"github.com/gorilla/mux"
	loggly "github.com/segmentio/go-loggly"
)

var (
	AgentHost     string
	Env           string
	VERSION       string // This is set when release_agent.sh is ran.
	InDevelopment = false
	Applications  = map[string]*Application{}
	streamManager = NewStreamManager()
	logClient     = loggly.New(config.LogglyKey, "agent")
)

func main() {
	ver := false
	runtime.GOMAXPROCS(1)
	flag.StringVar(&Env, "env", "production", "If you want to run the agent in development mode uses different ports")
	flag.BoolVar(&ver, "version", false, "Print the version")
	flag.Parse()
	if Env == "development" {
		InDevelopment = true
	}
	if ver {
		fmt.Println(VERSION)
		return
	}

	// Get host
	AgentHost, _ = util.GetHost()

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
	streamManager.pluginErr = pluginManager.Error
	go streamManager.Stream()

	// Add saved applications.
	LoadApps()
	for _, app := range Applications {
		<-Restart(app, true, true)
	}

	log.Println("agent starting")
	go logClient.Info("agent starting", map[string]interface{}{
		"version": VERSION,
		"arch":    runtime.GOARCH,
		"os":      runtime.GOOS,
		"ip":      AgentHost,
	})

	err := server.ListenAndServe()
	if err != nil {
		go logClient.Error(err.Error(), map[string]interface{}{
			"ip": AgentHost,
		})
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
