package cmd

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/dyng/nosdaily/database"
	"github.com/dyng/nosdaily/nostr"
	"github.com/dyng/nosdaily/service"
)

type Application struct {
	neo4j   *database.Neo4jDb
	service *service.Service
	crawler *nostr.Crawler
}

func NewApplication() *Application {
	neo4j := database.NewNeo4jDb()
	service := service.NewService(neo4j)
	crawler := nostr.NewCrawler(service)
	return &Application{
		neo4j:   neo4j,
		service: service,
		crawler: crawler,
	}
}

func (app *Application) Run() {
	app.neo4j.Connect()
	defer app.neo4j.Close()

	app.crawler.AddRelay("ws://localhost:10080")

	app.listenAndServe()
}

func (app *Application) listenAndServe() {
	mux := http.NewServeMux()
	mux.HandleFunc("/feed", app.handleFeed)

	err := http.ListenAndServe(":8080", mux)
	if errors.Is(err, http.ErrServerClosed) {
		log.Println("Server closed")
	} else {
		log.Printf("Server error:%v\n", err)
	}
}

func (app *Application) handleFeed(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(app.service.GetFeed())
}
