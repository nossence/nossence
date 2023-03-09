package nostr

import (
	"context"

	"github.com/dyng/nosdaily/service"
	"github.com/nbd-wtf/go-nostr"
	"github.com/ethereum/go-ethereum/log"
)

type Crawler struct {
	service *service.Service
	relaySubs map[string]*nostr.Subscription
}

func NewCrawler(service *service.Service) *Crawler {
	return &Crawler{
		service: service,
		relaySubs: make(map[string]*nostr.Subscription),
	}
}

func (c *Crawler) AddRelay(url string) {
	log.Info("Adding a relay server", "url", url)
	err := c.subscribe(url)
	if err != nil {
		log.Error("Failed to subscribe to relay", "url", url, "err", err)
	}
}

func (c *Crawler) subscribe(url string) error {
	relay, err := nostr.RelayConnect(context.Background(), url)
	if err != nil {
		return err
	}

	// TODO: make relaySubs thread safe
	sub := relay.Subscribe(context.Background(), []nostr.Filter{{
		Kinds: []int{1, 7, 30023},
		Limit: 100,
	}})
	c.relaySubs[url] = sub

	go func() {
		for ev := range sub.Events {
			log.Debug("Received event", "id", ev.ID, "kind", ev.Kind, "author", ev.PubKey, "created_at", ev.CreatedAt)
			err := c.service.StoreEvent(ev)
			if err != nil {
				log.Error("Failed to store event", "event", ev, "err", err)
			}
		}
	}()

	return nil
}
