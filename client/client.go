// Copyright 2014 Bowery, Inc.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/Bowery/gopackages/config"
	"github.com/Bowery/gopackages/rollbar"
	"github.com/Bowery/gopackages/web"
)

var (
	env              string
	port             string
	containerManager *ContainerManager
	rollbarC         *rollbar.Client
	AbsPath          string
	VERSION          string // This is set when release_client.sh is ran.
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

	go ssePool.run()

	rollbarC = rollbar.NewClient(config.RollbarToken, env)
	containerManager = NewContainerManager()
	defer containerManager.Close()

	go func() {
		for {
			select {
			case ev := <-containerManager.Syncer.Event:
				log.Println(ev)
				msg := map[string]interface{}{
					"event": ev,
					"type":  "sync",
				}
				ssePool.messages <- msg
			case err := <-containerManager.Syncer.Error:
				log.Println(err)
			}
		}
	}()

	abs, _ := filepath.Abs(filepath.Join(filepath.Dir(os.Args[0]), "../ui/"))
	AbsPath = abs

	server := web.NewServer(port, []web.Handler{
		new(web.SlashHandler),
		new(web.CorsHandler),
	}, routes)
	server.AuthHandler = &web.AuthHandler{Auth: web.DefaultAuthHandler}

	server.ListenAndServe()
}
