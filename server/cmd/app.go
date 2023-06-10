package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	"github.com/natefinch/lumberjack"
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

type ApiResponse struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data"`
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
	app.service.Init()
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
	mux.HandleFunc("/api/v1/recommendations/trends", app.handleRecommendationsTrends)
	mux.HandleFunc("/feed", app.handleFeed)
	mux.HandleFunc("/push", app.handlePush)
	mux.HandleFunc("/batch", app.handleBatch)
	mux.HandleFunc("/run", app.handleRun)
	mux.HandleFunc("/subscribe", app.handleSubscribe)
	mux.HandleFunc("/.well-known/nostr.json", app.nserver.Serve)

	log.Info("Server started")
	err := http.ListenAndServe(":8080", mux)
	if errors.Is(err, http.ErrServerClosed) {
		log.Info("Server closed")
	} else {
		log.Error("Server error", "err", err)
	}
}

func (app *Application) handleRecommendationsTrends(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()

	start, err := time.Parse(time.RFC3339, params.Get("startDateTime"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		doApiResponse(w, false, "startDateTime must be a valid ISO8601 string")
		return
	}

	end, err := time.Parse(time.RFC3339, params.Get("endDateTime"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		doApiResponse(w, false, "endDateTime must be a valid ISO8601 string")
		return
	}

	duration := end.Sub(start)
	if duration < time.Second || duration > time.Hour*24 {
		w.WriteHeader(http.StatusBadRequest)
		doApiResponse(w, false, "startDateTime and endDateTime must have difference between 1 second and 1 day")
		return
	}

	limit := 10
	limitOverride := params.Get("limit")
	if limitOverride != "" {
		limit, err := strconv.Atoi(limitOverride)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			doApiResponse(w, false, "limit must be a number between 1 and 100")
			return
		}
		if limit < 1 || limit > 100 {
			w.WriteHeader(http.StatusBadRequest)
			doApiResponse(w, false, "limit must be a number between 1 and 100")
			return
		}
	}

	feed, err := app.service.GetRecommendationsTrends(start, end, limit)
	if err != nil {
		doApiResponse(w, false, err.Error())
		return
	}
	doApiResponse(w, true, feed)
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
	useRepostParam := r.URL.Query().Get("useRepost")

	useRepost := true
	if useRepostParam != "" {
		useRepost, _ = strconv.ParseBool(useRepostParam)
	}

	subscriber := app.service.GetSubscriber(subscriberPub)
	if subscriber == nil {
		doResponse(w, false, "subscriber not found")
	}

	app.bot.Worker.Push(r.Context(), subscriberPub, subscriber.ChannelSecret, time.Hour, 10, useRepost)
	doResponse(w, true, "pushed")
}

func (app *Application) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	subscriberPub := r.URL.Query().Get("pubkey")
	ctx := context.Background()
	ba := app.bot

	channelSK, new, err := ba.Bot.GetOrCreateSubscription(ctx, subscriberPub)
	if err != nil {
		log.Warn("failed to create channel", "pubkey", subscriberPub, "err", err)
	}

	if new {
		err := ba.Bot.SendWelcomeMessage(ctx, channelSK, subscriberPub)
		if err != nil {
			log.Error("failed to send welcome message", "pubkey", subscriberPub, "err", err)
		} else {
			log.Info("sent welcome message to new subscriber", "pubkey", subscriberPub)
		}
	}

	// prepare initial content for first subscription
	err = ba.Worker.Push(ctx, subscriberPub, channelSK, bot.PushInterval, bot.PushSize, false)
	if err != nil {
		log.Error("failed to prepare initial content", "pubkey", subscriberPub, "err", err)
	}
	doResponse(w, true, "subscribed as pubkey "+subscriberPub)
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

func doApiResponse(w http.ResponseWriter, success bool, body any) {
	resp := ApiResponse{
		Status: "success",
		Data:   body,
	}

	if !success {
		resp.Status = "error"
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
	var wr io.Writer

	path := config.Log.Path
	if path == "console" {
		wr = os.Stdout
	} else {
		wr = &lumberjack.Logger{
			Filename: path,
			MaxSize:  config.Log.MaxSize, // megabytes
			MaxAge:   config.Log.MaxAge,  // days
			Compress: true,
		}
	}

	handler := log.StreamHandler(wr, log.TerminalFormat(false))
	level, _ := log.LvlFromString(config.Log.Level)
	handler = log.LvlFilterHandler(level, handler)
	log.Root().SetHandler(handler)
}
