// Copyright 2014 Bowery, Inc.
package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/Bowery/gopackages/config"
	"github.com/Bowery/gopackages/web"
	"github.com/unrolled/render"
)

var (
	isProduction bool
	port         string
	renderer     *render.Render
	routes       []web.Route
)

func init() {
	port = os.Getenv("PORT")

	isProduction = (os.Getenv("ENV") == "production")

	if isProduction {
		port = "80"
	}
	if port == "" {
		port = "2000"
	}
	if []rune(port)[0] != ':' {
		port = ":" + port
	}

	renderer = render.New(render.Options{
		IndentJSON:    true,
		IsDevelopment: !isProduction,
	})

	routes = []web.Route{
		{"GET", "/", helloHandler, false},
		{"GET", "/healthz", healthzHandler, false},
	}
}

func helloHandler(rw http.ResponseWriter, req *http.Request) {
	renderer.JSON(rw, http.StatusOK, map[string]string{
		"code_name": "Mercer",
		"purpose":   "Sync Dev Envs Across Teams",
	})
}

func healthzHandler(rw http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(rw, "ok")
}

func main() {
	web.NewServer(port, []web.Handler{
		new(web.SlashHandler),
		new(web.CorsHandler),
		&web.StatHandler{Key: config.StatHatKey, Name: "mercer"},
	}, routes).ListenAndServe()
}
