// Copyright 2013-2014 Bowery, Inc.
package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Bowery/gopackages/localdb"
	"github.com/Bowery/gopackages/schemas"
	"github.com/Bowery/gopackages/sys"
	"github.com/codegangsta/negroni"
	"github.com/unrolled/render"
)

var (
	AuthEndpoint   string = "broome.io"
	DaemonEndpoint string = "localhost:3000" // TODO (thebyrd) change this to match the toolbar app
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
	// Update local
	return db.Save(data)
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", indexHandler)
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
	r.HTML(rw, http.StatusOK, "home", map[string]interface{}{
		"Title":  "Home Page!",
		"Status": "All Systems Go!",
	})
}

func appsHandler(rw http.ResponseWriter, req *http.Request) {
	r.HTML(rw, http.StatusOK, "applications", map[string]interface{}{
		"Title": "Applications",
		"Apps":  getApps(),
	})
}

func newAppHandler(rw http.ResponseWriter, req *http.Request) {
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
