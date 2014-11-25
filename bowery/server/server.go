// Copyright 2014 Bowery, Inc.
package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"crypto/hmac"
	"crypto/sha256"

	"code.google.com/p/go-uuid/uuid"
	"github.com/Bowery/gopackages/config"
	"github.com/Bowery/gopackages/requests"
	"github.com/Bowery/gopackages/web"
	"github.com/orchestrate-io/gorc"
	"github.com/unrolled/render"
)

var (
	isProduction bool
	port         string
	renderer     *render.Render
	routes       []web.Route
	db           *gorc.Client
)

func init() {
	// parse environment variables
	isProduction = (os.Getenv("ENV") == "production")
	port = os.Getenv("PORT")

	// choose port
	if isProduction {
		port = "80"
	}
	if port == "" {
		port = "2000"
	}
	if []rune(port)[0] != ':' {
		port = ":" + port
	}

	// create renderer
	renderer = render.New(render.Options{
		IndentJSON:    true,
		IsDevelopment: !isProduction,
	})

	// define routes
	routes = []web.Route{
		{"GET", "/", helloHandler, false},
		{"GET", "/healthz", healthzHandler, false},
		{"POST", "/signup", signupHandler, false},
	}

	// create orchestrate client
	orchestrateKey := config.OrchestrateDevKey
	if isProduction {
		orchestrateKey = config.OrchestrateProdKey
	}
	db = gorc.NewClient(orchestrateKey)
}

func missingFieldResponse(rw http.ResponseWriter, field string) {
	renderer.JSON(rw, http.StatusBadRequest, map[string]string{
		"status": requests.STATUS_FAILED,
		"error":  field + " is a required field",
	})
}

// must equal the node implementation:
// > require('crypto').createHmac('sha256', 'hello').update('world').digest('hex')
// 'f1ac9702eb5faf23ca291a4dc46deddeee2a78ccdaf0a412bed7714cfffb1cc4'
func hashPassword(password, salt string) string {
	hash := hmac.New(sha256.New, []byte(salt))
	hash.Write([]byte(password))
	return hex.EncodeToString(hash.Sum(nil))
}

func helloHandler(rw http.ResponseWriter, req *http.Request) {
	renderer.JSON(rw, http.StatusOK, map[string]string{
		"code_name": "Mercer",
		"purpose":   "Sync Dev Envs Across Teams.",
	})
}

func healthzHandler(rw http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(rw, "ok")
}

func signupHandler(rw http.ResponseWriter, req *http.Request) {
	body := map[string]string{}
	decoder := json.NewDecoder(req.Body)

	if err := decoder.Decode(&body); err != nil {
		fmt.Fprintf(rw, err.Error())
		return
	}

	// check for required fields
	for _, field := range []string{"email", "password"} {
		if body[field] == "" {
			missingFieldResponse(rw, field)
			return
		}
	}

	dev := new(Developer)
	dev.ID = uuid.New()
	dev.Email = body["email"]
	dev.Salt = uuid.New()
	dev.Password = hashPassword(body["password"], dev.Salt)

	// if _, err := db.Put("developers", dev.ID, dev); err != nil {
	// 	renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
	// 		"status": requests.STATUS_FAILED,
	// 		"error":  err.Error(),
	// 	})
	// 	return
	// }

	team := new(Team)
	team.ID = uuid.New()
	team.Creator = dev
	team.Members = make([]*Developer, 0)
	team.Members = append(team.Members, dev)
	team.Path = make([]string, 0)
	for _, path := range strings.Split(os.Getenv("PATH"), ":") {
		team.Path = append(team.Path, path)
		err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			fmt.Println("Looking at " + path) // TODO (thebyrd) md5 sum, upload to s3, & build environment.
			return nil
		})
		if err != nil {
			renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
				"status": requests.STATUS_FAILED,
				"error":  "Failed to analyze local environment: " + err.Error(),
			})
		}
	}

	// if _, err := db.Put("teams", team.ID, team); err != nil {
	// 	renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
	// 		"status": requests.STATUS_FAILED,
	// 		"error":  err.Error(),
	// 	})
	// 	return
	// }

	fmt.Fprintf(rw, "I still need to create the master Env from my local Env")
}

func main() {
	web.NewServer(port, []web.Handler{
		new(web.SlashHandler),
		new(web.CorsHandler),
		// &web.StatHandler{Key: config.StatHatKey, Name: "mercer"},
	}, routes).ListenAndServe()
}
