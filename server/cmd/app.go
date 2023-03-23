package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/dyng/nosdaily/bot"
	"github.com/dyng/nosdaily/database"
	"github.com/dyng/nosdaily/nostr"
	"github.com/dyng/nosdaily/service"
	"github.com/dyng/nosdaily/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/omeid/uconfig"
)

type Application struct {
	config  *types.Config
	neo4j   *database.Neo4jDb
	service *service.Service
	crawler *nostr.Crawler
	bot     *bot.BotApplication
	nserver *nostr.NameServer
}

type response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
}

func NewApplication() *Application {
	// load config and init logger
	config := loadConfig()
	initLogger(config)
	log.Debug("Loaded configuration", "config", config)

	// inject dependencies
	neo4j := database.NewNeo4jDb(config)
	service := service.NewService(config, neo4j)
	crawler := nostr.NewCrawler(config, service)
	bot := bot.NewBotApplication(config, service)
	nserver := nostr.NewNameServer(config, neo4j)
	return &Application{
		config:  config,
		neo4j:   neo4j,
		service: service,
		crawler: crawler,
		bot:     bot,
		nserver: nserver,
	}
}

func (app *Application) Run() {
	// connect to neo4j
	err := app.neo4j.Connect()
	if err != nil {
		log.Crit("Failed to connect to neo4j", "err", err)
	}
	app.service.InitDatabase()
	defer app.neo4j.Close()

	// start crawler
	app.crawler.Run()

	// start bot app
	go func() {
		app.bot.Run(context.Background())
	}()

	// start http server
	app.listenAndServe()
}

func (app *Application) listenAndServe() {
	mux := http.NewServeMux()
	mux.HandleFunc("/feed", app.handleFeed)
	mux.HandleFunc("/push", app.handlePush)
	mux.HandleFunc("/batch", app.handleBatch)
	mux.HandleFunc("/run", app.handleRun)
	mux.HandleFunc("/.well-known/nostr.json", app.nserver.Serve)

	log.Info("Server started")
	err := http.ListenAndServe(":8080", mux)
	if errors.Is(err, http.ErrServerClosed) {
		log.Info("Server closed")
	} else {
		log.Error("Server error", "err", err)
	}
}

func (app *Application) handleFeed(w http.ResponseWriter, r *http.Request) {
	userPub := r.URL.Query().Get("pubkey")

	feed := app.service.GetFeed(userPub, time.Now().Add(-1*time.Hour), time.Now(), 10)
	doResponse(w, true, feed)
}

func (app *Application) handleRun(w http.ResponseWriter, r *http.Request) {
	app.bot.Worker.Run(r.Context())
	doResponse(w, true, "dispatched")
}

func (app *Application) handleBatch(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	skip, _ := strconv.Atoi(r.URL.Query().Get("skip"))
	app.bot.Worker.Batch(r.Context(), limit, skip)
	doResponse(w, true, "dispatched")
}

func (app *Application) handlePush(w http.ResponseWriter, r *http.Request) {
	subscriberPub := r.URL.Query().Get("pubkey")

	subscriber := app.service.GetSubscriber(subscriberPub)
	if subscriber == nil {
		doResponse(w, false, "subscriber not found")
	}

	app.bot.Worker.Push(r.Context(), subscriberPub, subscriber.ChannelSecret, time.Hour, 10)
	doResponse(w, true, "pushed")
}

func doResponse(w http.ResponseWriter, success bool, body any) {
	resp := response{
		Success: success,
		Data:    body,
	}

	err := json.NewEncoder(w).Encode(resp)
	if err != nil {
		log.Error("Failed to encode response body", "body", body, "err", err)
	}
}

func loadConfig() *types.Config {
	config := &types.Config{}
	files := uconfig.Files{
		{"config.json", json.Unmarshal},
	}
	_, err := uconfig.Classic(&config, files)
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}
	setDefaultValue(config)
	return config
}

// set some default values that cannot be parsed by struct tags
func setDefaultValue(config *types.Config) {
	if config.Bot.Metadata.About == "" {
		config.Bot.Metadata.About = "A recommender engine for nostr. Follow this account and post '@nossence #subscribe' to get your own feed!"
	}

	if config.Bot.Metadata.ChannelAbout == "" {
		config.Bot.Metadata.ChannelAbout = "nossence curated content for %s powered by %s"
	}
}

func initLogger(config *types.Config) {
	// log to console by default
	file := os.Stdout

	path := config.Log.Path
	if path != "console" {
		var err error
		file, err = os.OpenFile(path, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			fmt.Printf("Cannot create log file at path %s: %v\n", path, err)
			os.Exit(1)
		}
	}

	handler := log.StreamHandler(file, log.TerminalFormat(false))
	level, _ := log.LvlFromString(config.Log.Level)
	handler = log.LvlFilterHandler(level, handler)
	log.Root().SetHandler(handler)
}
