// Copyright 2013-2014 Bowery, Inc.
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Bowery/gopackages/localdb"
	"github.com/Bowery/gopackages/schemas"
	"github.com/Bowery/gopackages/sys"
	"github.com/codegangsta/negroni"
	"github.com/unrolled/render"
)

var (
	AuthEndpoint   string = "http://broome.io"
	DaemonEndpoint string = "http://localhost:3000" // TODO (thebyrd) change this to match the toolbar app
	db             *localdb.DB
	data           *localData
)

var r = render.New(render.Options{
	IndentJSON:    true,
	IsDevelopment: true,
	Layout:        "layout",
})

type Application struct {
	ID         string
	Name       string
	Start      string
	Build      string
	Env        map[string]string
	RemotePath string
	RemoteAddr string
	LocalPath  string
}

const (
	AuthCreateTokenPath = "/developers/token"
	AuthMePath          = "/developers/me?token={token}"
)

type localData struct {
	Developer    *schemas.Developer
	Applications []*Application
}

// Set up local db.
func init() {
	var err error
	db, err = localdb.New(filepath.Join(os.Getenv(sys.HomeVar), ".bowery_state"))
	if err != nil {
		log.Println("Unable to create local database.")
		return
	}

	data = new(localData)
	if err = db.Load(data); err == io.EOF || os.IsNotExist(err) {
		log.Println("No existing state")
	}
}

func getApps() []*Application {
	return data.Applications
}

func getAppById(id string) *Application {
	var application Application
	for _, app := range getApps() {
		if app.ID == id {
			application = *app
			break
		}
	}

	return &application
}

func getDev() *schemas.Developer {
	return data.Developer
}

func updateDev() error {
	// todo(steve): update broome
	return db.Save(data)
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", indexHandler)
	mux.HandleFunc("/signup", signupHandler)
	mux.HandleFunc("/_/signup", createDeveloperHandler)
	mux.HandleFunc("/login", loginHandler)
	mux.HandleFunc("/_/login", submitLoginHandler)
	mux.HandleFunc("/logout", logoutHandler)
	mux.HandleFunc("/apps", appsHandler)
	mux.HandleFunc("/applications/new", newAppHandler)
	mux.HandleFunc("/applications/verify", verifyAppHandler)
	mux.HandleFunc("/applications/create", createAppHandler)
	mux.HandleFunc("/applications/", appHandler)
	mux.HandleFunc("/settings", getSettingsHandler)
	mux.HandleFunc("/_/settings", updateSettingsHandler)

	app := negroni.Classic()
	app.UseHandler(mux)
	app.Run(":3001")
}

func indexHandler(rw http.ResponseWriter, req *http.Request) {
	// If there is no logged in user, show login page.
	if getDev().ID.Hex() == "" {
		http.Redirect(rw, req, "/login", http.StatusTemporaryRedirect)
		return
	}

	http.Redirect(rw, req, "/apps", http.StatusTemporaryRedirect)
}

func signupHandler(rw http.ResponseWriter, req *http.Request) {
	// If there is no logged in user, show login page.
	dev := getDev()
	if dev != nil && getDev().ID.Hex() != "" {
		http.Redirect(rw, req, "/apps", http.StatusTemporaryRedirect)
		return
	}

	r.HTML(rw, http.StatusOK, "signup", map[string]string{
		"Title": "Welcome to Bowery",
	})
}

func loginHandler(rw http.ResponseWriter, req *http.Request) {
	// If there is no logged in user, show login page.
	dev := getDev()
	if dev != nil && getDev().ID.Hex() != "" {
		http.Redirect(rw, req, "/apps", http.StatusTemporaryRedirect)
		return
	}

	r.HTML(rw, http.StatusOK, "login", map[string]string{
		"Title": "Login to Bowery",
	})
}

func logoutHandler(rw http.ResponseWriter, req *http.Request) {
	data.Developer = &schemas.Developer{}
	db.Save(data)
	http.Redirect(rw, req, "/login", http.StatusTemporaryRedirect)
}

type loginReq struct {
	Email    string
	Password string
}

type res struct {
	Status string `json:"status"`
	Err    string `json:"error"`
}

type createTokenRes struct {
	*res
	Token string `json:"token"`
}

type developerRes struct {
	*res
	Developer *schemas.Developer `json:"developer"`
}

func submitLoginHandler(rw http.ResponseWriter, req *http.Request) {
	email := req.FormValue("email")
	password := req.FormValue("password")

	// To login a user, first fetch their token, and then
	// using their token, get the developer object.

	// Get token.
	var body bytes.Buffer
	bodyReq := &loginReq{
		Email:    email,
		Password: password,
	}
	encoder := json.NewEncoder(&body)
	if err := encoder.Encode(bodyReq); err != nil {
		r.HTML(rw, http.StatusBadRequest, "error", map[string]interface{}{
			"Error": err.Error(),
		})
		return
	}

	res, err := http.Post(AuthEndpoint+AuthCreateTokenPath, "application/json", &body)
	if err != nil {
		r.HTML(rw, http.StatusBadRequest, "error", map[string]interface{}{
			"Error": err.Error(),
		})
		return
	}
	defer res.Body.Close()

	// Decode response.
	createRes := new(createTokenRes)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(createRes)
	if err != nil {
		r.HTML(rw, http.StatusBadRequest, "error", map[string]interface{}{
			"Error": err.Error(),
		})
		return
	}

	token := ""
	if createRes.Status == "created" {
		token = createRes.Token
	}

	// Get developer.
	res, err = http.Get(AuthEndpoint + strings.Replace(AuthMePath, "{token}", token, -1))
	if err != nil {
		r.HTML(rw, http.StatusBadRequest, "error", map[string]interface{}{
			"Error": err.Error(),
		})
		return
	}
	defer res.Body.Close()

	// Decode response.
	devRes := new(developerRes)
	decoder = json.NewDecoder(res.Body)
	err = decoder.Decode(devRes)
	if err != nil {
		r.HTML(rw, http.StatusBadRequest, "error", map[string]interface{}{
			"Error": err.Error(),
		})
		return
	}

	if devRes.Status == "found" {
		data.Developer = devRes.Developer
		db.Save(data)

		// Redirect to applications.
		http.Redirect(rw, req, "/apps", http.StatusTemporaryRedirect)
		return
	}

	// todo(steve) handle error.
}

func createDeveloperHandler(rw http.ResponseWriter, req *http.Request) {
	name := req.FormValue("name")
	email := req.FormValue("email")
	password := req.FormValue("password")

	if name == "" || email == "" || password == "" {
		r.HTML(rw, http.StatusBadRequest, "signup", map[string]interface{}{
			"Error": "Missing fields",
		})
		return
	}
}

func appsHandler(rw http.ResponseWriter, req *http.Request) {
	// If there is no logged in user, show login page.
	dev := getDev()
	if dev == nil || dev.ID.Hex() == "" {
		http.Redirect(rw, req, "/login", http.StatusTemporaryRedirect)
		return
	}

	r.HTML(rw, http.StatusOK, "applications", map[string]interface{}{
		"Title": "Applications",
		"Apps":  getApps(),
	})
}

func newAppHandler(rw http.ResponseWriter, req *http.Request) {
	// If there is no logged in user, show login page.
	dev := getDev()
	if dev == nil || dev.ID.Hex() == "" {
		http.Redirect(rw, req, "/login", http.StatusTemporaryRedirect)
		return
	}

	r.HTML(rw, http.StatusOK, "new", map[string]interface{}{
		"Title": "New Application",
	})
}

func verifyAppHandler(rw http.ResponseWriter, req *http.Request) {
	r.JSON(rw, http.StatusOK, map[string]string{"todo": "true"})
}

func createAppHandler(rw http.ResponseWriter, req *http.Request) {
	r.JSON(rw, http.StatusOK, map[string]string{"todo": "true"})
}

func appHandler(rw http.ResponseWriter, req *http.Request) {
	// If there is no logged in user, show login page.
	dev := getDev()
	if dev == nil || dev.ID.Hex() == "" {
		http.Redirect(rw, req, "/login", http.StatusTemporaryRedirect)
		return
	}

	id := req.URL.Path[len("/applications/"):]
	application := getAppById(id)

	if application.ID == "" {
		r.HTML(rw, http.StatusBadRequest, "error", map[string]string{
			"Error": "No such application.",
		})
		return
	}

	r.HTML(rw, http.StatusOK, "application", map[string]interface{}{
		"Application": application,
		"Status":      "Syncing...",
	})
}

func getSettingsHandler(rw http.ResponseWriter, req *http.Request) {
	// If there is no logged in user, show login page.
	dev := getDev()
	if dev == nil || dev.ID.Hex() == "" {
		http.Redirect(rw, req, "/login", http.StatusTemporaryRedirect)
		return
	}

	r.HTML(rw, http.StatusOK, "settings", map[string]interface{}{
		"Developer": getDev(),
	})
}

func updateSettingsHandler(rw http.ResponseWriter, req *http.Request) {
	name := req.FormValue("name")
	email := req.FormValue("email")
	dev := getDev()

	if name != "" {
		dev.Name = name
	}

	if email != "" {
		dev.Email = email
	}

	if err := updateDev(); err != nil {
		r.JSON(rw, http.StatusOK, map[string]string{
			"status": "failed",
			"error":  err.Error(),
		})
		return
	}

	r.JSON(rw, http.StatusOK, map[string]string{
		"status": "success",
	})
}
