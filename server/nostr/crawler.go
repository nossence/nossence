package nostr

import (
	"context"
	"strconv"
	"time"

	"github.com/dyng/nosdaily/service"
	"github.com/dyng/nosdaily/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/nbd-wtf/go-nostr"
)

type Crawler struct {
	config      *types.Config
	service     *service.Service
	connections map[string]*relayConnection
}

func NewCrawler(config *types.Config, service *service.Service) *Crawler {
	return &Crawler{
		config:      config,
		service:     service,
		connections: make(map[string]*relayConnection),
	}
}

func (c *Crawler) Run() {
	log.Info("Starting crawler")
	for _, url := range c.config.Crawler.Relays {
		c.AddRelay(url)
	}
}

func (c *Crawler) AddRelay(url string) {
	go func() {
		log.Info("Adding a relay server", "url", url)

		since := time.Now().Add(parseTimeOffset(c.config.Crawler.Since))
		limit := c.config.Crawler.Limit
		conn, err := c.subscribe(url, since, limit)
		if err != nil {
			log.Error("Failed to subscribe to relay", "url", url, "err", err)
			return
		}

		for {
			select {
			case err := <-conn.error:
				log.Info("Close & reconnect to relay", "url", url)
				err = conn.Close()
				if err != nil {
					log.Error("Failed to close connection", "url", url, "err", err)
				}

				// wait for a while
				waitPeriod := 30 * time.Second
				time.Sleep(waitPeriod)

				// reconnect
				conn, err = c.subscribe(url, time.Now().Add(-waitPeriod), 1000)
				if err != nil {
					log.Error("Failed to subscribe to relay", "url", url, "err", err)
					return
				}
				log.Info("Reconnected to relay", "url", url)
			}
		}
	}()
}

func (c *Crawler) subscribe(url string, since time.Time, limit int) (*relayConnection, error) {
	ctx, cancel := context.WithCancel(context.Background())

	relay, err := nostr.RelayConnect(ctx, url)
	if err != nil {
		cancel()
		return nil, err
	}

	var filter nostr.Filter
	if limit != 0 {
		filter = nostr.Filter{
			Kinds: []int{1, 3, 6, 7, 9735},
			Since: &since,
			Limit: limit,
		}
	} else {
		filter = nostr.Filter{
			Kinds: []int{1, 3, 6, 7, 9735},
			Since: &since,
		}
	}
	log.Debug("Subscribing to relay", "url", url, "filter", filter)
	sub := relay.Subscribe(ctx, []nostr.Filter{filter})

	conn := relayConnection{
		relay:  relay,
		cancel: cancel,
		error:  make(chan error),
	}
	c.connections[url] = &conn

	go func() {
		for {
			select {
			case ev := <-sub.Events:
				log.Debug("Received event", "id", ev.ID, "kind", ev.Kind, "author", ev.PubKey, "created_at", ev.CreatedAt)
				err := c.service.StoreEvent(ev)
				if err != nil {
					log.Error("Failed to store event", "event", ev, "err", err)
				}
			case notice := <-relay.Notices:
				log.Warn("Received relay notice", "notice", notice)
			case err := <-relay.ConnectionError:
				log.Error("Connection error", "url", url, "err", err)
				conn.error <- err
			case <-ctx.Done():
				log.Debug("Stop consuming events", "url", url)
				return
			}
		}
	}()

	return &conn, nil
}

type relayConnection struct {
	relay  *nostr.Relay
	cancel context.CancelFunc
	error  chan error
}

func (rc *relayConnection) Close() error {
	rc.cancel() // 'cancel' is used to close the subscription
	return rc.relay.Close()
}

func parseTimeOffset(offset string) time.Duration {
	// split offset into number and unit
	num, unit := offset[:len(offset)-1], offset[len(offset)-1:]
	n, err := strconv.Atoi(num)
	if err != nil {
		log.Error("Failed to parse time offset", "offset", offset, "err", err)
		return 0
	}

	switch unit {
	case "s":
		return time.Duration(n) * time.Second
	case "m":
		return time.Duration(n) * time.Minute
	case "h":
		return time.Duration(n) * time.Hour
	case "d":
		return time.Duration(n) * time.Hour * 24
	default:
		log.Error("Unknown time unit", "unit", unit)
		return 0
	}
}
