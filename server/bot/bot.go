package bot

import (
	"context"
	"fmt"

	n "github.com/dyng/nosdaily/nostr"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

type Bot struct {
	client *n.Client
	sk     string
	pub    string
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
