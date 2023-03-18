package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	n "github.com/dyng/nosdaily/nostr"
	"github.com/dyng/nosdaily/service"
	"github.com/dyng/nosdaily/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/robfig/cron/v3"
)

var logger = log.New("module", "bot")

type BotApplication struct {
	Bot    *Bot
	config *types.Config
	Worker *Worker
}

type Bot struct {
	client  *n.Client
	service *service.Service
	SK      string
	pub     string
}

func NewBotApplication(config *types.Config, service *service.Service) *BotApplication {
	ctx := context.Background()

	client, err := n.NewClient(ctx, config.Bot.Relays)
	if err != nil {
		panic(err)
	}

	bot, err := NewBot(ctx, client, service, config.Bot.SK)
	if err != nil {
		panic(err)
	}

	worker, err := NewWorker(ctx, client, service)
	if err != nil {
		panic(err)
	}

	return &BotApplication{
		Bot:    bot,
		config: config,
		Worker: worker,
	}
}

func (ba *BotApplication) Run(ctx context.Context) error {
	c, err := ba.Bot.Listen(ctx)
	if err != nil {
		logger.Crit("cannot listen to subscribe messages", "err", err)
	}

	logger.Info("start listening to subscribe messages...")

	done := make(chan struct{})
	defer close(done)

	go func(c <-chan nostr.Event) {
		for ev := range c {
			logger.Info("received event", "event", ev.Content)
			if strings.Contains(ev.Content, "#subscribe") {
				logger.Info("preparing channel for user", "pubkey", ev.PubKey)
				subSK, new, err := ba.Bot.GetOrCreateSubSK(ctx, ev.PubKey)
				if err != nil {
					logger.Warn("failed to create channel for user", "pubkey", ev.PubKey, "err", err)
					continue
				}

				if new {
					ba.Bot.SendWelcomeMessage(ctx, subSK, ev.PubKey)
					logger.Info("sent welcome message to new user", "pubkey", ev.PubKey)
				} else {
					logger.Info("known user, check if it is resubscription", "pubkey", ev.PubKey)
					restored, err := ba.Bot.RestoreSubSK(ctx, ev.PubKey)
					if err != nil {
						logger.Warn("failed to restore subscription", "pubkey", ev.PubKey, "err", err)
					}

					if restored {
						ba.Bot.SendWelcomeMessage(ctx, subSK, ev.PubKey)
						logger.Info("sent welcome message to resubscribing user", "pubkey", ev.PubKey)
					}
				}
			} else if strings.Contains(ev.Content, "#unsubscribe") {
				logger.Warn("unsubscribing user", "pubkey", ev.PubKey)
				ba.Bot.RemoveSubSK(ctx, ev.PubKey)
			}
		}

		done <- struct{}{}
	}(c)

	cr := cron.New()
	cr.AddFunc("0 * * * *", func() {
		logger.Info("running hourly cron job")
		ba.Worker.Batch(ctx, 100, 0) // TODO: should check next
	})

	<-done
	cr.Stop()
	logger.Info("bot exiting...")
	return nil
}

func NewBot(ctx context.Context, client *n.Client, service *service.Service, sk string) (*Bot, error) {
	pub, err := nostr.GetPublicKey(sk)
	if err != nil {
		return nil, err
	}

	return &Bot{
		client:  client,
		SK:      sk,
		pub:     pub,
		service: service,
	}, nil
}

func (b *Bot) Listen(ctx context.Context) (<-chan nostr.Event, error) {
	now := time.Now()
	filters := nostr.Filters{
		nostr.Filter{
			Kinds: []int{1},
			Since: &now,
			Tags: nostr.TagMap{
				"p": []string{b.pub},
			},
		},
	}
	return b.client.Subscribe(ctx, filters), nil
}

func (b *Bot) GetOrCreateSubSK(ctx context.Context, userPub string) (string, bool, error) {
	subscriber := b.service.GetSubscriber(userPub)
	if subscriber != nil {
		return subscriber.ChannelSecret, false, nil
	}

	subSK := nostr.GeneratePrivateKey()
	err := b.service.CreateSubscriber(userPub, subSK, time.Now())
	if err != nil {
		return "", false, err
	}

	return subSK, true, nil
}

func (b *Bot) RemoveSubSK(ctx context.Context, userPub string) error {
	return b.service.DeleteSubscriber(userPub, time.Now())
}

func (b *Bot) RestoreSubSK(ctx context.Context, userPub string) (bool, error) {
	return b.service.RestoreSubscriber(userPub, time.Now())
}

func (b *Bot) SendWelcomeMessage(ctx context.Context, subSK, receiverPub string) error {
	receiverNpub, err := nip19.EncodePublicKey(receiverPub)
	if err != nil {
		return err
	}

	subPub, err := nostr.GetPublicKey(subSK)
	if err != nil {
		return err
	}
	subNpub, err := nip19.EncodePublicKey(subPub)
	if err != nil {
		return err
	}

	msg := fmt.Sprintf("Hello, %s! Your nossence recommendations is ready, follow: %s to fetch your own feed.", receiverNpub, subNpub)
	return b.client.SendMessage(ctx, b.SK, receiverPub, msg)
}
