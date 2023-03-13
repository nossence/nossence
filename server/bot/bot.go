package bot

import (
	"context"
	"fmt"
	"strings"

	n "github.com/dyng/nosdaily/nostr"
	"github.com/dyng/nosdaily/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

var botlog = log.New("module", "bot")
var userSubStore = make(map[string]string)

type BotApplication struct {
	bot    *Bot
	config *types.Config
}

type Bot struct {
	client *n.Client
	sk     string
	pub    string
}

func NewBotApplication(config *types.Config) *BotApplication {
	ctx := context.Background()

	client, err := n.NewClient(ctx, config.Bot.Relays)
	if err != nil {
		panic(err)
	}

	bot, err := NewBot(ctx, client, config.Bot.SK)
	if err != nil {
		panic(err)
	}

	return &BotApplication{
		bot:    bot,
		config: config,
	}
}

func (ba *BotApplication) Run(ctx context.Context) error {
	c, err := ba.bot.Listen(ctx)
	if err != nil {
		botlog.Crit("cannot listen to subscribe messages", "err", err)
	}

	botlog.Info("start listening to subscribe messages...")

	done := make(chan struct{})
	defer close(done)

	go func(c <-chan nostr.Event) {
		for ev := range c {
			if strings.Contains(ev.Content, "#subscribe") {
				botlog.Info("preparing channel for user", "pubkey", ev.PubKey)
				_, new, err := ba.bot.GetOrCreateSubSK(ctx, ev.PubKey)
				if err != nil {
					botlog.Warn("failed to create channel for user", "pubkey", ev.PubKey, "err", err)
					continue
				}

				if new {
					ba.bot.SendWelcomeMessage(ctx, ba.config.Bot.SK, ev.PubKey)
					botlog.Info("sent welcome message to new user", "pubkey", ev.PubKey)
				} else {
					botlog.Info("known user, skipping welcome message", "pubkey", ev.PubKey)
				}
			}
		}

		done <- struct{}{}
	}(c)

	<-done
	botlog.Info("bot exiting...")
	return nil
}

func NewBot(ctx context.Context, client *n.Client, sk string) (*Bot, error) {
	pub, err := nostr.GetPublicKey(sk)
	if err != nil {
		return nil, err
	}

	return &Bot{
		client: client,
		sk:     sk,
		pub:    pub,
	}, nil
}

func (b *Bot) Listen(ctx context.Context) (<-chan nostr.Event, error) {
	filters := nostr.Filters{
		nostr.Filter{
			Kinds: []int{1},
			Tags: nostr.TagMap{
				"p": []string{b.pub},
			},
		},
	}
	return b.client.Subscribe(ctx, filters), nil
}

func (b *Bot) GetOrCreateSubSK(ctx context.Context, userPub string) (string, bool, error) {
	// TODO: lock in case there're multiple attempts on the same pubkey
	// TODO: use a persistent storage for subSK
	if subSK, ok := userSubStore[userPub]; ok {
		return subSK, false, nil
	}

	subSK := nostr.GeneratePrivateKey()
	userSubStore[userPub] = subSK
	return subSK, true, nil
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
	return b.client.SendMessage(ctx, b.sk, receiverPub, msg)
}
