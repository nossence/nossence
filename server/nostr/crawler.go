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

const (
	ReconnectInterval = 3600 // 1 hour
)

type Crawler struct {
	config      *types.Config
	service     *service.Service
	connections map[string]*relayConn
}

func NewCrawler(config *types.Config, service *service.Service) *Crawler {
	return &Crawler{
		config:      config,
		service:     service,
		connections: make(map[string]*relayConn),
	}
}

func (c *Crawler) Run() {
	log.Info("Starting crawler")
	for _, url := range c.config.Crawler.Relays {
		c.AddRelay(url)
	}
}

func (c *Crawler) AddRelay(url string) {
	log.Info("Adding a relay server", "url", url)

	since := time.Now().Add(parseTimeOffset(c.config.Crawler.Since))
	limit := c.config.Crawler.Limit
	err := c.subscribe(url, since, limit)
	if err != nil {
		log.Error("Failed to subscribe to relay", "url", url, "err", err)
	}

	go func() {
		for {
			timer := time.NewTicker(ReconnectInterval * time.Second)
			<-timer.C

			// reconnect
			log.Info("Close & reconnect to relay", "url", url)
			err := c.connections[url].Close()
			if err != nil {
				log.Error("Failed to close connection", "url", url, "err", err)
				continue
			}
			c.subscribe(url, time.Now(), 100)
		}
	}()
}

func (c *Crawler) subscribe(url string, since time.Time, limit int) error {
	relay, err := nostr.RelayConnect(context.Background(), url)
	if err != nil {
		return err
	}

	var filter nostr.Filter
	if limit != 0 {
		filter = nostr.Filter{
			Kinds: []int{1, 3, 7, 9735},
			Since: &since,
			Limit: limit,
		}
	} else {
		filter = nostr.Filter{
			Kinds: []int{1, 3, 7, 9735},
			Since: &since,
		}
	}
	log.Debug("Subscribing to relay", "url", url, "filter", filter)
	sub := relay.Subscribe(context.Background(), []nostr.Filter{filter})

	conn := relayConn{
		relay: relay,
		sub:   sub,
		done:  make(chan bool, 1),
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
			case <-conn.done:
				log.Debug("Stop consuming events", "url", url)
				return
			}
		}
	}()

	return nil
}

type relayConn struct {
	relay *nostr.Relay
	sub   *nostr.Subscription
	done  chan bool
}

func (rc *relayConn) Close() error {
	rc.done <- true
	rc.sub.Unsub()
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
