package nostr

import (
	"context"
	"log"

	"github.com/dyng/nosdaily/service"
	"github.com/nbd-wtf/go-nostr"
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
	// TODO: handle error
	c.subscribe(url)
}

func (c *Crawler) subscribe(url string) error {
	relay, err := nostr.RelayConnect(context.Background(), url)
	if err != nil {
		return err
	}

	// TODO: make relaySubs thread safe
	sub := relay.Subscribe(context.Background(), []nostr.Filter{{
		Kinds: []int{1, 30023},
		Limit: 100,
	}})
	c.relaySubs[url] = sub

	go func() {
		for ev := range sub.Events {
			log.Printf("Received event: id=%s, kind=%d, created_at=%s\n", ev.ID, ev.Kind, ev.CreatedAt)
			err := c.service.StoreEvent(ev)
			if err != nil {
				log.Printf("Error storing event: %v\n", err)
			}
		}
	}()

	return nil
}
