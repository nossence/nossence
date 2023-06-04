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
	"golang.org/x/exp/slices"
)

var logger = log.New("module", "bot")

const (
	PushInterval = time.Hour
	PushSize     = 5
)

type BotApplication struct {
	Bot    *Bot
	config *types.Config
	Worker *Worker
}

type Bot struct {
	client  n.IClient
	service service.IService
	config  *types.Config
	SK      string
	pub     string
}

func NewBotApplication(config *types.Config, service *service.Service) *BotApplication {
	ctx := context.Background()

	client, err := n.NewClient(ctx, config.Bot.Relays)
	if err != nil {
		panic(err)
	}

	bot, err := NewBot(ctx, client, service, config)
	if err != nil {
		panic(err)
	}

	worker, err := NewWorker(ctx, client, service, config)
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

	schedule := "0 * * * *"
	logger.Info("register worker cron job", "schedule", schedule)

	cr := cron.New()
	defer cr.Stop()
	cr.AddFunc(schedule, func() {
		logger.Info("running cron job")
		ba.Worker.Run(ctx)
	})
	cr.Start()

	logger.Info("start listening to subscribe messages...")

	done := make(chan struct{})
	defer close(done)

	go func(c <-chan nostr.Event) {
		for ev := range c {
			logger.Info("received mentioning event", "event", ev.Content)
			if strings.Contains(ev.Content, "#subscribe") {
				logger.Info("preparing channel", "pubkey", ev.PubKey)
				channelSK, new, err := ba.Bot.GetOrCreateSubscription(ctx, ev.PubKey)
				if err != nil {
					logger.Warn("failed to create channel", "pubkey", ev.PubKey, "err", err)
					continue
				}

				if new {
					err := ba.Bot.SendWelcomeMessage(ctx, channelSK, ev.PubKey)
					if err != nil {
						logger.Error("failed to send welcome message", "pubkey", ev.PubKey, "err", err)
					} else {
						logger.Info("sent welcome message to new subscriber", "pubkey", ev.PubKey)
					}
				} else {
					restored, err := ba.Bot.RestoreSubscription(ctx, ev.PubKey)
					if err != nil {
						logger.Warn("failed to restore subscription", "pubkey", ev.PubKey, "err", err)
					}

					if restored {
						logger.Info("sending welcome message to returning subscriber", "pubkey", ev.PubKey)
						err := ba.Bot.SendWelcomeMessage(ctx, channelSK, ev.PubKey)
						if err != nil {
							logger.Warn("failed to send welcome message returning subscriber", "pubkey", ev.PubKey, "err", err)
						}
					} else {
						logger.Info("skip welcome message for existing subscriber", "pubkey", ev.PubKey)
					}
				}

				// prepare initial content for first subscription
				err = ba.Worker.Push(ctx, ev.PubKey, channelSK, PushInterval, PushSize, false)
				if err != nil {
					logger.Error("failed to prepare initial content", "pubkey", ev.PubKey, "err", err)
				}
			} else if strings.Contains(ev.Content, "#unsubscribe") {
				logger.Warn("unsubscribing", "pubkey", ev.PubKey)
				ba.Bot.TerminateSubscription(ctx, ev.PubKey)
			}
		}

		done <- struct{}{}
	}(c)

	<-done
	cr.Stop()
	logger.Info("bot exiting...")
	return nil
}

func NewBot(ctx context.Context, client n.IClient, service service.IService, config *types.Config) (*Bot, error) {
	sk := config.Bot.SK
	pub, err := nostr.GetPublicKey(sk)
	if err != nil {
		return nil, err
	}

	return &Bot{
		client:  client,
		config:  config,
		SK:      sk,
		pub:     pub,
		service: service,
	}, nil
}

func (b *Bot) Listen(ctx context.Context) (<-chan nostr.Event, error) {
	// set user metadata
	logger.Info("Create account metadata", "pubkey", b.pub)
	metadata := b.config.Bot.Metadata
	relays := b.recommendedRelayList(*b.config)
	err := b.client.Metadata(ctx, b.SK, metadata.Name, metadata.About, metadata.Picture, metadata.Nip05, relays)
	if err != nil {
		logger.Error("failed to set account metadata", "err", err)
	}

	// listen to subscription message
	logger.Info("Listen to subscription message", "pubkey", b.pub)
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

func (b *Bot) GetOrCreateSubscription(ctx context.Context, subscriberPub string) (string, bool, error) {
	subscriber := b.service.GetSubscriber(subscriberPub)
	if subscriber != nil {
		logger.Info("found existing subscriber", "pubkey", subscriberPub)
		return subscriber.ChannelSecret, false, nil
	}

	logger.Info("creating new subscriber", "pubkey", subscriberPub)
	channelSK, err := b.createSubscription(ctx, subscriberPub)
	if err != nil {
		return "", false, err
	}

	return channelSK, true, nil
}

func (b *Bot) createSubscription(ctx context.Context, subscriberPub string) (string, error) {
	metadata := b.config.Bot.Metadata
	channelSK := nostr.GeneratePrivateKey()

	// save secret key to db
	err := b.service.CreateSubscriber(subscriberPub, channelSK, time.Now())
	if err != nil {
		return "", err
	}

	// send set_metadata event
	npub, _ := nip19.EncodePublicKey(subscriberPub)
	mainNpub, _ := nip19.EncodePublicKey(b.pub)
	relays := b.recommendedRelayList(*b.config)
	err = b.client.Metadata(ctx, channelSK,
		metadata.ChannelName,
		fmt.Sprintf(metadata.ChannelAbout, npub, mainNpub),
		metadata.ChannelPicture, "", relays)
	if err != nil {
		return "", err
	}

	return channelSK, nil
}

func (b *Bot) TerminateSubscription(ctx context.Context, subscriberPub string) error {
	return b.service.DeleteSubscriber(subscriberPub, time.Now())
}

func (b *Bot) RestoreSubscription(ctx context.Context, subscriberPub string) (bool, error) {
	return b.service.RestoreSubscriber(subscriberPub, time.Now())
}

func (b *Bot) SendWelcomeMessage(ctx context.Context, channelSK, receiverPub string) error {
	channelPub, err := nostr.GetPublicKey(channelSK)
	if err != nil {
		return err
	}

	msg := "Hello, #[0]! Your nossence curator is ready, follow: #[1] to fetch your own feed."
	return b.client.Mention(ctx, b.SK, msg, []string{
		receiverPub,
		channelPub,
	})
}

func (b *Bot) recommendedRelayList(config types.Config) []types.RelayInfo {
	relays := []types.RelayInfo{}

	for _, r := range config.Bot.Relays {
		if slices.Contains(config.Crawler.Relays, r) {
			relays = append(relays, types.RelayInfo{
				URL:     r,
				Purpose: "",
			})
		} else {
			relays = append(relays, types.RelayInfo{
				URL:     r,
				Purpose: "write",
			})
		}
	}

	for _, r := range config.Crawler.Relays {
		if !slices.Contains(config.Bot.Relays, r) {
			relays = append(relays, types.RelayInfo{
				URL:     r,
				Purpose: "read",
			})
		}
	}

	return relays
}
