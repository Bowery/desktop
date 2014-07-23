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
	"time"

	"code.google.com/p/go-uuid/uuid"
	"github.com/Bowery/gopackages/localdb"
	"github.com/Bowery/gopackages/schemas"
	"github.com/Bowery/gopackages/sys"
	"github.com/codegangsta/negroni"
	"github.com/gorilla/websocket"
	"github.com/unrolled/render"
)

var (
	AuthEndpoint   = "http://broome.io"
	DaemonEndpoint = "http://localhost:3000" // TODO (thebyrd) change this to match the toolbar app
	syncer         = NewSyncer()
	logManager     = NewLogManager()
	db             *localdb.DB
	data           *localData
)

var r = render.New(render.Options{
	IndentJSON:    true,
	IsDevelopment: true,
	Layout:        "layout",
})

type Application struct {
	ID            string
	Name          string
	Start         string
	Build         string
	Env           map[string]string
	RemotePath    string
	RemoteAddr    string
	LocalPath     string
	LastUpdatedAt time.Time
}

const (
	AuthCreateDeveloperPath = "/developers"
	AuthCreateTokenPath     = "/developers/token"
	AuthMePath              = "/developers/me?token={token}"
)

type localData struct {
	Developer    *schemas.Developer
	Applications []*Application
}

type wsError struct {
	Application *Application `json:"application"`
	Err         string       `json:"error"`
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
		return
	}
}

func broadcastJSON(data interface{}) {
	msg, err := json.Marshal(data)
	if err != nil {
		msg = []byte(`{"error": "` + strings.Replace(err.Error(), `"`, "'", -1) + `"}`)
	}

	wsPool.broadcast <- msg
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
	defer syncer.Close()
	defer logManager.Close()

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
	mux.HandleFunc("/applications/update", updateAppHandler)
	mux.HandleFunc("/applications/remove", removeAppHandler)
	mux.HandleFunc("/applications/", appHandler)
	mux.HandleFunc("/settings", getSettingsHandler)
	mux.HandleFunc("/_/settings", updateSettingsHandler)
	mux.HandleFunc("/_/ws", wsHandler)

	// Start ws
	go wsPool.run()

	// Start retrieving sync events.
	go func() {
		for {
			select {
			case ev := <-syncer.Event:
				broadcastJSON(ev)
			case err := <-syncer.Error:
				ws := new(wsError)
				we, ok := err.(*WatchError)
				if !ok {
					ws.Err = err.Error()
				} else {
					ws.Application = we.Application
					ws.Err = we.Err.Error()
				}

				broadcastJSON(ws)
			}
		}
	}()

	if data.Applications != nil {
		for _, app := range data.Applications {
			syncer.Watch(app)
			logManager.Connect(app)
			broadcastJSON(&Event{Application: app, Status: "upload-start"})
		}
	}

	app := negroni.Classic()
	app.UseHandler(mux)
	app.Run(":3001")
}

func indexHandler(rw http.ResponseWriter, req *http.Request) {
	// If there is no logged in user, show login page.
	dev := getDev()
	if dev == nil || dev.Token == "" {
		http.Redirect(rw, req, "/login", http.StatusTemporaryRedirect)
		return
	}

	http.Redirect(rw, req, "/apps", http.StatusTemporaryRedirect)
}

func signupHandler(rw http.ResponseWriter, req *http.Request) {
	// If there is no logged in user, show login page.
	dev := getDev()
	if dev != nil && dev.Token != "" {
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
	if dev != nil && dev.Token != "" {
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
	Name     string
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

	var body bytes.Buffer
	bodyReq := &loginReq{Name: name, Email: email, Password: password}

	encoder := json.NewEncoder(&body)
	err := encoder.Encode(bodyReq)
	if err != nil {
		r.HTML(rw, http.StatusBadRequest, "error", map[string]interface{}{
			"Error": err.Error(),
		})
		return
	}

	res, err := http.Post(AuthEndpoint+AuthCreateDeveloperPath, "application/json", &body)
	if err != nil {
		r.HTML(rw, http.StatusBadRequest, "error", map[string]interface{}{
			"Error": err.Error(),
		})
		return
	}
	defer res.Body.Close()

	// Decode json response.
	createRes := new(developerRes)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(createRes)
	if err != nil {
		r.HTML(rw, http.StatusBadRequest, "error", map[string]interface{}{
			"Error": err.Error(),
		})
		return
	}

	// Created, just return token.
	if createRes.Status == "created" {
		data.Developer = createRes.Developer
		db.Save(data)

		http.Redirect(rw, req, "/apps", http.StatusTemporaryRedirect)
		return
	}

	if strings.Contains(createRes.Err, "email already exists") {
		http.Redirect(rw, req, "/signup?error=emailtaken", http.StatusTemporaryRedirect)
		return
	}

	http.Redirect(rw, req, "/signup", http.StatusTemporaryRedirect)
	return
}

func appsHandler(rw http.ResponseWriter, req *http.Request) {
	// If there is no logged in user, show login page.
	dev := getDev()
	if dev == nil || dev.Token == "" {
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
	if dev == nil || dev.Token == "" {
		http.Redirect(rw, req, "/login", http.StatusTemporaryRedirect)
		return
	}

	r.HTML(rw, http.StatusOK, "new", map[string]interface{}{
		"Title": "New Application",
	})
}

func verifyAppHandler(rw http.ResponseWriter, req *http.Request) {
	requestProblems := map[string]string{}

	remoteAddr := req.FormValue("ip-addr")
	err := DelanceyCheck(remoteAddr)
	if err != nil {
		requestProblems["ip-addr"] = remoteAddr + " delancey endpoint can't be reached."
	}

	localDir := req.FormValue("local-dir")
	if len(localDir) >= 2 && localDir[:2] == "~/" {
		localDir = strings.Replace(localDir, "~", os.Getenv(sys.HomeVar), 1)
	}
	if stat, err := os.Stat(localDir); os.IsNotExist(err) || !stat.IsDir() {
		requestProblems["local-dir"] = localDir + " is not a valid directory."
	}

	r.JSON(rw, http.StatusOK, requestProblems)
}

func createAppHandler(rw http.ResponseWriter, req *http.Request) {
	localDir := req.FormValue("local-dir")
	if len(localDir) >= 2 && localDir[:2] == "~/" {
		localDir = strings.Replace(localDir, "~", os.Getenv(sys.HomeVar), 1)
	}

	app := &Application{
		ID:            uuid.New(),
		Name:          req.FormValue("name"),
		Start:         req.FormValue("start"),
		Build:         req.FormValue("build"),
		RemotePath:    req.FormValue("remote-dir"),
		RemoteAddr:    req.FormValue("ip-addr"),
		LocalPath:     localDir,
		LastUpdatedAt: time.Now(),
	}

	if data.Applications == nil {
		data.Applications = []*Application{}
	}

	data.Applications = append(data.Applications, app)
	db.Save(data)

	syncer.Watch(app)
	logManager.Connect(app)
	broadcastJSON(&Event{Application: app, Status: "upload-start"})

	r.JSON(rw, http.StatusOK, map[string]interface{}{"success": true})
}

func updateAppHandler(rw http.ResponseWriter, req *http.Request) {
	app := getAppById(req.FormValue("id"))
	if app.ID == "" {
		r.HTML(rw, http.StatusBadRequest, "error", map[string]string{
			"Error": "No such application.",
		})
		return
	}

	localDir := req.FormValue("local-dir")
	if len(localDir) >= 2 && localDir[:2] == "~/" {
		localDir = strings.Replace(localDir, "~", os.Getenv(sys.HomeVar), 1)
	}

	app.Name = req.FormValue("name")
	app.Start = req.FormValue("start")
	app.Build = req.FormValue("build")
	app.RemotePath = req.FormValue("remote-dir")
	app.RemoteAddr = req.FormValue("ip-addr")
	app.LocalPath = localDir
	app.LastUpdatedAt = time.Now()
	for i, a := range data.Applications {
		if a.ID == app.ID {
			data.Applications[i] = app
		}
	}
	db.Save(data)

	syncer.Remove(app)
	logManager.Remove(app)
	syncer.Watch(app)
	logManager.Connect(app)
	broadcastJSON(&Event{Application: app, Status: "upload-start"})

	r.JSON(rw, http.StatusOK, map[string]interface{}{
		"success": true,
		"app":     app,
	})
}

func removeAppHandler(rw http.ResponseWriter, req *http.Request) {
	apps := getApps()
	for i, app := range apps {
		if app.ID == req.FormValue("id") {
			apps[i], apps[len(apps)-1], apps = apps[len(apps)-1], nil, apps[:len(apps)-1] // Fancy Remove
			break
		}
	}
	data.Applications = apps
	db.Save(data)
	r.JSON(rw, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

func appHandler(rw http.ResponseWriter, req *http.Request) {
	// If there is no logged in user, show login page.
	dev := getDev()
	if dev == nil || dev.Token == "" {
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
		"Title":       application.Name,
		"Application": application,
		"Status":      "Syncing...",
	})
}

func getSettingsHandler(rw http.ResponseWriter, req *http.Request) {
	// If there is no logged in user, show login page.
	dev := getDev()
	if dev == nil || dev.Token == "" {
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

var upgrader = &websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024}

func wsHandler(rw http.ResponseWriter, req *http.Request) {
	ws, err := upgrader.Upgrade(rw, req, nil)
	if err != nil {
		return
	}

	conn := &connection{
		send: make(chan []byte, 256),
		ws:   ws,
	}

	wsPool.register <- conn

	defer func() {
		wsPool.unregister <- conn
	}()

	go conn.writer()
	conn.reader()
}
