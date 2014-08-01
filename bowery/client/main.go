// Copyright 2013-2014 Bowery, Inc.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"code.google.com/p/go-uuid/uuid"
	"github.com/Bowery/desktop/bowery/client/bpm"
	"github.com/Bowery/gopackages/config"
	"github.com/Bowery/gopackages/keen"
	"github.com/Bowery/gopackages/localdb"
	"github.com/Bowery/gopackages/schemas"
	"github.com/Bowery/gopackages/sys"
	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/unrolled/render"
)

var (
	AuthEndpoint = "http://broome.io"
	syncer       = NewSyncer()
	logManager   = NewLogManager()
	db           *localdb.DB
	data         *localData
	dbDir        = filepath.Join(os.Getenv(sys.HomeVar), ".bowery", "state")
	logDir       = filepath.Join(os.Getenv(sys.HomeVar), ".bowery", "logs")
	keenC        *keen.Client
)

var r = render.New(render.Options{
	IndentJSON:    true,
	IsDevelopment: true,
	Layout:        "layout",
})

var externalViewRenderer = render.New(render.Options{
	IsDevelopment: true,
})

type Application struct {
	ID              string
	Name            string
	Start           string
	Build           string
	RemotePath      string
	RemoteAddr      string
	SyncPort        string
	LogPort         string
	LocalPath       string
	LastUpdatedAt   time.Time
	IsSyncAvailable bool
}

const (
	AuthCreateDeveloperPath = "/developers"
	AuthCreateTokenPath     = "/developers/token"
	AuthUpdateDeveloperPath = "/developers/{token}"
	AuthMePath              = "/developers/me?token={token}"
	AuthResetPasswordPath   = "/reset/{email}"
)

type localData struct {
	Developer    *schemas.Developer
	Applications []*Application
}

type wsError struct {
	Application *Application `json:"application"`
	Err         string       `json:"error"`
}

type Route struct {
	Method  string
	Path    string
	Handler http.HandlerFunc
}

// SlashHandler is a http.Handler that removes trailing slashes.
type SlashHandler struct {
	Handler http.Handler
}

// ServeHTTP strips trailing slashes and calls the handler.
func (sh *SlashHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		req.URL.Path = strings.TrimRight(req.URL.Path, "/")
		req.RequestURI = req.URL.RequestURI()
	}

	sh.Handler.ServeHTTP(rw, req)
}

// Set up local db.
func init() {
	if os.Getenv("AGENT") == "development" {
		// You'll have a seperate user/applications when using the dev agent
		dbDir = filepath.Join(os.Getenv(sys.HomeVar), ".bowery", "devstate")
	}

	var err error
	db, err = localdb.New(dbDir)
	if err != nil {
		log.Println("Unable to create local database.")
		return
	}

	keenC = &keen.Client{
		WriteKey:  config.KeenWriteKey,
		ProjectID: config.KeenProjectID,
	}

	data = new(localData)
	if err = db.Load(data); err == io.EOF || os.IsNotExist(err) {
		// Get developer.
		data.Developer = &schemas.Developer{}
		db.Save(data)
	}

	cwd, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	if err := os.Chdir(cwd); err != nil {
		panic("Wrong Directory!")
	}

	// Make sure log dir is created
	os.MkdirAll(logDir, os.ModePerm|os.ModeDir)

	go func() {
		for {
			<-time.After(5 * time.Second)
			if data.Applications == nil {
				continue
			}

			for _, app := range data.Applications {
				status := "connect"
				if err := DelanceyCheck(app.RemoteAddr + ":" + app.SyncPort); err != nil {
					status = "disconnect"
				}

				broadcastJSON(&Event{Application: app, Status: status})
			}
		}
	}()
}

func main() {
	defer syncer.Close()
	defer logManager.Close()

	var Routes = []*Route{
		&Route{"GET", "/", indexHandler},
		&Route{"GET", "/signup", signupHandler},
		&Route{"POST", "/signup", createDeveloperHandler},
		&Route{"GET", "/login", loginHandler},
		&Route{"POST", "/login", submitLoginHandler},
		&Route{"GET", "/logout", logoutHandler},
		&Route{"GET", "/reset", resetHandler},
		&Route{"POST", "/reset", submitResetHandler},
		&Route{"GET", "/pause", pauseSyncHandler},
		&Route{"GET", "/resume", resumeSyncHandler},
		&Route{"GET", "/applications", appsHandler},
		&Route{"GET", "/applications/new", newAppHandler},
		&Route{"POST", "/applications/verify", verifyAppHandler},
		&Route{"POST", "/applications", createAppHandler},
		&Route{"POST", "/applications/{id}/plugins/{version}", addPluginHandler},
		&Route{"PUT", "/applications/{id}", updateAppHandler},
		&Route{"DELETE", "/applications/{id}", removeAppHandler},
		&Route{"GET", "/applications/{id}", appHandler},
		&Route{"GET", "/plugins", listPluginsHandler},
		&Route{"GET", "/plugins/{version}", showPluginHandler},
		&Route{"GET", "/logs/{id}", logsHandler},
		&Route{"GET", "/settings", getSettingsHandler},
		&Route{"POST", "/settings", updateSettingsHandler},
		&Route{"GET", "/_/ws", wsHandler},
	}

	router := mux.NewRouter()
	for _, r := range Routes {
		route := router.NewRoute()
		route.Path(r.Path).Methods(r.Method)
		route.HandlerFunc(r.Handler)
	}

	// Start ws
	go wsPool.run()

	// Start retrieving sync events.
	go func() {
		for {
			select {
			case ev := <-syncer.Event:
				keenC.AddEvent("bowery/desktop sync", map[string]interface{}{
					"user":  data.Developer,
					"event": ev,
				})
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
	app.UseHandler(&SlashHandler{router})

	port := os.Getenv("PORT")
	if port == "" {
		port = "32055"
	}
	app.Run(":" + port)
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

func getToken() error {
	// Get token.
	var body bytes.Buffer
	bodyReq := &loginReq{
		Email:    data.Developer.Email,
		Password: data.Developer.Password,
	}
	encoder := json.NewEncoder(&body)
	if err := encoder.Encode(bodyReq); err != nil {
		return err
	}

	res, err := http.Post(AuthEndpoint+AuthCreateTokenPath, "application/json", &body)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// Decode response.
	createRes := new(createTokenRes)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(createRes)
	if err != nil {
		return err
	}

	if createRes.Status == "created" {
		data.Developer.Token = createRes.Token
	}

	db.Save(data)

	return nil
}

func getDev() *schemas.Developer {
	res, err := http.Get(AuthEndpoint + strings.Replace(AuthMePath, "{token}", data.Developer.Token, -1))
	if err != nil {
		return data.Developer
	}
	defer res.Body.Close()

	// Decode response.
	devRes := new(developerRes)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(devRes)
	if err != nil {
		return data.Developer
	}

	if devRes.Status != "found" {
		if err = getToken(); err != nil {
			return data.Developer
		}
	}

	data.Developer = devRes.Developer

	return data.Developer
}

func updateDev(oldpass, newpass string) error {
	form := make(url.Values)
	form.Set("name", data.Developer.Name)
	form.Set("email", data.Developer.Email)
	form.Set("oldpassword", oldpass)
	form.Set("password", newpass)

	url := strings.Replace(AuthUpdateDeveloperPath, "{token}", data.Developer.Token, -1)
	req, err := http.NewRequest("PUT", AuthEndpoint+url, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(data.Developer.Token, "")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Decode json response.
	updateRes := new(updateDeveloperRes)
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(updateRes)
	if err != nil {
		return err
	}

	if updateRes.Status != "updated" {
		return updateRes
	}

	pass, ok := updateRes.Update["password"]
	if ok {
		data.Developer.Password = pass.(string)
	}

	keenC.AddEvent("bowery/desktop user update", map[string]*schemas.Developer{"user": data.Developer})
	return db.Save(data)
}

func indexHandler(rw http.ResponseWriter, req *http.Request) {
	// If there is no logged in user, show login page.
	dev := data.Developer
	if dev == nil || dev.Token == "" {
		http.Redirect(rw, req, "/login", http.StatusSeeOther)
		return
	}

	r.HTML(rw, http.StatusOK, "home", map[string]string{
		"Title": "Bowery",
	})
}

func signupHandler(rw http.ResponseWriter, req *http.Request) {
	// If there is no logged in user, show login page.
	dev := data.Developer
	if dev != nil && dev.Token != "" {
		http.Redirect(rw, req, "/applications", http.StatusSeeOther)
		return
	}

	r.HTML(rw, http.StatusOK, "signup", map[string]string{
		"Title": "Welcome to Bowery",
	})
}

func loginHandler(rw http.ResponseWriter, req *http.Request) {
	// If there is no logged in user, show login page.
	dev := data.Developer
	if dev != nil && dev.Token != "" {
		http.Redirect(rw, req, "/applications", http.StatusSeeOther)
		return
	}

	r.HTML(rw, http.StatusOK, "login", map[string]string{
		"Title": "Login to Bowery",
	})
}

func logoutHandler(rw http.ResponseWriter, req *http.Request) {
	data.Developer = &schemas.Developer{}
	db.Save(data)
	http.Redirect(rw, req, "/login", http.StatusSeeOther)
}

func resetHandler(rw http.ResponseWriter, req *http.Request) {
	r.HTML(rw, http.StatusOK, "reset", map[string]string{
		"Title": "Reset Your Password",
	})
}

func submitResetHandler(rw http.ResponseWriter, req *http.Request) {
	email := req.FormValue("email")
	if email == "" {
		r.HTML(rw, http.StatusBadRequest, "error", map[string]interface{}{
			"Error": "Missing fields",
		})
		return
	}

	resp, err := http.Get(AuthEndpoint + strings.Replace(AuthResetPasswordPath, "{email}", email, -1))
	if err != nil {
		r.HTML(rw, http.StatusBadRequest, "error", map[string]interface{}{
			"Error": err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	resetRes := new(res)
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(resetRes)
	if err != nil {
		r.HTML(rw, http.StatusBadRequest, "error", map[string]interface{}{
			"Error": err.Error(),
		})
		return
	}

	if resetRes.Status == "success" {
		http.Redirect(rw, req, "/login", http.StatusSeeOther)
		return
	}

	r.HTML(rw, http.StatusBadRequest, "error", map[string]interface{}{
		"Error": resetRes.Error(),
	})
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

func (res *res) Error() string {
	return res.Err
}

type createTokenRes struct {
	*res
	Token string `json:"token"`
}

type developerRes struct {
	*res
	Developer *schemas.Developer `json:"developer"`
}

type updateDeveloperRes struct {
	*res
	Update map[string]interface{} `json:"update"`
}

func submitLoginHandler(rw http.ResponseWriter, req *http.Request) {
	if data.Developer == nil {
		data.Developer = &schemas.Developer{}
	}

	email := req.FormValue("email")
	password := req.FormValue("password")

	data.Developer.Email = email
	data.Developer.Password = password

	if err := getToken(); err != nil {
		r.HTML(rw, http.StatusBadRequest, "error", map[string]interface{}{
			"Error": err.Error(),
		})
	}

	data.Developer = getDev()

	db.Save(data)

	keenC.AddEvent("bowery/desktop login", map[string]*schemas.Developer{"user": data.Developer})
	// Redirect to applications.
	http.Redirect(rw, req, "/applications", http.StatusSeeOther)
}

func createDeveloperHandler(rw http.ResponseWriter, req *http.Request) {
	name := req.FormValue("name")
	email := req.FormValue("email")
	password := req.FormValue("password")

	if name == "" || email == "" || password == "" {
		r.HTML(rw, http.StatusBadRequest, "error", map[string]interface{}{
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

		http.Redirect(rw, req, "/applications", http.StatusSeeOther)
		return
	}

	if strings.Contains(createRes.Error(), "email already exists") {
		http.Redirect(rw, req, "/signup?error=emailtaken", http.StatusSeeOther)
		return
	}

	keenC.AddEvent("bowery/desktop signup", map[string]*schemas.Developer{"user": data.Developer})
	http.Redirect(rw, req, "/signup", http.StatusSeeOther)
	return
}

func pauseSyncHandler(rw http.ResponseWriter, req *http.Request) {
	if data.Applications == nil {
		return
	}

	for _, app := range data.Applications {
		syncer.Remove(app)
		logManager.Remove(app)
	}

	keenC.AddEvent("bowery/desktop sync pause", map[string]*schemas.Developer{"user": data.Developer})
	r.JSON(rw, http.StatusOK, map[string]interface{}{"success": true})
}

func resumeSyncHandler(rw http.ResponseWriter, req *http.Request) {
	if data.Applications == nil {
		return
	}

	for _, app := range data.Applications {
		syncer.Watch(app)
		logManager.Connect(app)
		broadcastJSON(&Event{Application: app, Status: "upload-start"})
	}

	keenC.AddEvent("bowery/desktop sync resume", map[string]*schemas.Developer{"user": data.Developer})
	r.JSON(rw, http.StatusOK, map[string]interface{}{"success": true})
}

func appsHandler(rw http.ResponseWriter, req *http.Request) {
	// If there is no logged in user, show login page.
	dev := data.Developer
	if dev == nil || dev.Token == "" {
		http.Redirect(rw, req, "/login", http.StatusSeeOther)
		return
	}

	r.HTML(rw, http.StatusOK, "applications", map[string]interface{}{
		"Title": "Applications",
		"Apps":  getApps(),
	})
}

func newAppHandler(rw http.ResponseWriter, req *http.Request) {
	// If there is no logged in user, show login page.
	dev := data.Developer
	if dev == nil || dev.Token == "" {
		http.Redirect(rw, req, "/login", http.StatusSeeOther)
		return
	}

	r.HTML(rw, http.StatusOK, "new", map[string]interface{}{
		"Title": "New Application",
	})
}

func verifyAppHandler(rw http.ResponseWriter, req *http.Request) {
	requestProblems := map[string]string{}

	remoteAddr := req.FormValue("ip-addr")

	defaultSyncPort := ":3001"
	if os.Getenv("AGENT") == "development" {
		defaultSyncPort = ":3003"
	}

	var err error
	if len(strings.Split(remoteAddr, ":")) > 1 {
		err = DelanceyCheck(remoteAddr)
	} else {
		err = DelanceyCheck(remoteAddr + defaultSyncPort)
	}
	if err != nil {
		requestProblems["ip-addr"] = "http://" + remoteAddr + defaultSyncPort + " can't be reached."
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
		ID:              uuid.New(),
		Name:            req.FormValue("name"),
		Start:           req.FormValue("start"),
		Build:           req.FormValue("build"),
		RemotePath:      req.FormValue("remote-dir"),
		LocalPath:       localDir,
		LastUpdatedAt:   time.Now(),
		IsSyncAvailable: true,
	}

	// Parse address. Split into
	ipAddr := req.FormValue("ip-addr")
	hostAndPort := strings.Split(ipAddr, ":")
	if len(hostAndPort) == 1 {
		app.RemoteAddr = ipAddr
		app.SyncPort = "3001"
		if os.Getenv("AGENT") == "development" {
			app.SyncPort = "3003"
		}
	} else {
		app.RemoteAddr = hostAndPort[0]
		app.SyncPort = hostAndPort[1]
	}
	// Log Port is always SyncPort + 1
	sp, _ := strconv.Atoi(app.SyncPort)
	app.LogPort = strconv.Itoa(sp + 1)

	if data.Applications == nil {
		data.Applications = []*Application{}
	}

	data.Applications = append(data.Applications, app)
	db.Save(data)

	syncer.Watch(app)
	logManager.Connect(app)
	broadcastJSON(&Event{Application: app, Status: "upload-start"})

	keenC.AddEvent("bowery/desktop app new", map[string]*schemas.Developer{"user": data.Developer})
	r.JSON(rw, http.StatusOK, map[string]interface{}{"success": true})
}

func addPluginHandler(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	version := vars["version"]
	appId := vars["app"]

	fmt.Println(version, appId)

	var plugin string
	// Install Plugin
	for _, formula := range bpm.GetFormulae() {
		if formula.Version == version {
			plugin = formula.Name
			if err := bpm.InstallPlugin(formula.Name); err != nil {
				r.HTML(rw, http.StatusBadRequest, "error", map[string]string{
					"Error": err.Error(),
				})
				return
			}
			break
		}
	}
	fmt.Println(plugin)

	//TODO (thebyrd) Upload to Agent

	r.JSON(rw, http.StatusOK, map[string]bool{"success": true})
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
	appId := mux.Vars(req)["id"]
	apps := getApps()
	for i, app := range apps {
		if app.ID == appId {
			syncer.Remove(app)
			logManager.Remove(app)
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
	dev := data.Developer
	if dev == nil || dev.Token == "" {
		http.Redirect(rw, req, "/login", http.StatusSeeOther)
		return
	}
	application := getAppById(mux.Vars(req)["id"])

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

func listPluginsHandler(rw http.ResponseWriter, req *http.Request) {
	// If there is no logged in user, show login page.
	dev := data.Developer
	if dev == nil || dev.Token == "" {
		http.Redirect(rw, req, "/login", http.StatusSeeOther)
		return
	}

	plugins := bpm.GetFormulae()

	// TODO (thebyrd) get all plugins
	r.HTML(rw, http.StatusOK, "plugins", map[string]interface{}{
		"Title":   "Plugins",
		"Plugins": plugins,
	})

}

func showPluginHandler(rw http.ResponseWriter, req *http.Request) {
	// If there is no logged in user, show login page.
	dev := data.Developer
	if dev == nil || dev.Token == "" {
		http.Redirect(rw, req, "/login", http.StatusSeeOther)
		return
	}

	version := mux.Vars(req)["version"]

	for _, plugin := range bpm.GetFormulae() {
		if plugin.Version == version {
			r.HTML(rw, http.StatusOK, "plugin", map[string]interface{}{
				"Title":  plugin.Name,
				"Plugin": plugin,
				"Apps":   getApps(),
			})
			return
		}
	}
	// TODO (thebyrd) get all plugins
	r.HTML(rw, http.StatusOK, "error", map[string]interface{}{
		"Title": "Error",
		"Error": "Plugin not found. See http://github.com/bowery/plugins for a list of availble plugins.",
	})
}

func logsHandler(rw http.ResponseWriter, req *http.Request) {
	// Parse application ID.
	appID := mux.Vars(req)["id"]

	// Read from file.
	logs, err := ioutil.ReadFile(filepath.Join(logDir, appID+".log"))
	if err != nil {
		log.Println(err)
	}

	externalViewRenderer.HTML(rw, http.StatusOK, "logs", map[string]string{
		"Logs": string(bytes.Trim(logs, "\x00")),
	})
}

func getSettingsHandler(rw http.ResponseWriter, req *http.Request) {
	// If there is no logged in user, show login page.
	dev := getDev()
	if dev == nil || dev.Token == "" {
		http.Redirect(rw, req, "/login", http.StatusSeeOther)
		return
	}

	r.HTML(rw, http.StatusOK, "settings", map[string]interface{}{
		"Title":     "Settings",
		"Developer": dev,
	})
}

func updateSettingsHandler(rw http.ResponseWriter, req *http.Request) {
	name := req.FormValue("name")
	email := req.FormValue("email")
	oldpass := req.FormValue("oldpassword")
	newpass := req.FormValue("password")
	dev := data.Developer

	if name != "" {
		dev.Name = name
	}

	if email != "" {
		dev.Email = email
	}

	if err := updateDev(oldpass, newpass); err != nil {
		r.HTML(rw, http.StatusBadRequest, "error", map[string]interface{}{
			"Error": err.Error(),
		})
		return
	}

	http.Redirect(rw, req, "/applications", http.StatusSeeOther)
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
