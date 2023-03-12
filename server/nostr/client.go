package nostr

import (
	"context"
	"fmt"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip04"
	"github.com/nbd-wtf/go-nostr/nip19"
)

type Client struct {
	Relays []*nostr.Relay
}

func DecodeNsec(nsec string) (string, error) {
	prefix, val, err := nip19.Decode(nsec)
	if err != nil {
		return "", err
	}

	if prefix != "nsec" {
		return "", fmt.Errorf("invalid nsec prefix: %s", prefix)
	}

	if pub, ok := val.(string); ok {
		return pub, nil
	}

	return "", fmt.Errorf("invalid nsec value: %v", val)
}

func DecodeNpub(npub string) (string, error) {
	prefix, val, err := nip19.Decode(npub)
	if err != nil {
		return "", err
	}

	if prefix != "npub" {
		return "", fmt.Errorf("invalid npub prefix: %s", prefix)
	}

	if pub, ok := val.(string); ok {
		return pub, nil
	}

	return "", fmt.Errorf("invalid npub value: %v", val)
}

func NewClient(ctx context.Context, relays []string) (*Client, error) {
	rs := []*nostr.Relay{}
	for _, relay := range relays {
		r, err := nostr.RelayConnect(ctx, relay)
		if err != nil {
			return nil, err
		}
		rs = append(rs, r)
	}

	return &Client{
		Relays: rs,
	}, nil
}

// func (c *Client) Subscribe(ctx context.Context, filters []nostr.Filter) (nostr.Subscription, error) {
// 	sub := c.Subscribe(ctx, filters)
// 	return sub.Events, nil
// }

// Publish a signed event to all relays
func (c *Client) Publish(ctx context.Context, ev nostr.Event) error {
	for _, r := range c.Relays {
		status := r.Publish(ctx, ev)
		if status == nostr.PublishStatusFailed {
			return fmt.Errorf("publish failed")
		}
	}
	return nil
}

// Repost an event
func (c *Client) Repost(ctx context.Context, sk, eventID, authorPub string) error {
	pub, err := nostr.GetPublicKey(sk)
	if err != nil {
		return err
	}

	// There's ongoing disucssion about how to create a repost event:
	// https://github.com/nostr-protocol/nips/issues/173
	// And there's potential NIP-10 will be extended to support repost:
	// https://github.com/nostr-protocol/nips/pull/310
	//
	// For now aligning with iris.to implementation:
	// https://github.com/irislib/iris-messenger/blob/0c6181c4dfe3ef0dc96607bc00222b63a473321a/src/js/components/events/Note.js#L76-L83
	ev := nostr.Event{
		PubKey: pub,
		Kind:   6,
		Tags: nostr.Tags{
			nostr.Tag{"e", eventID, "", "mention"},
			nostr.Tag{"p", authorPub},
		},
		Content: "",
	}

	err = ev.Sign(sk)
	if err != nil {
		return err
	}

	return c.Publish(ctx, ev)
}

// Sends a NIP-04 message
func (c *Client) SendMessage(ctx context.Context, sk, receiverPub, msg string) error {
	senderPub, err := nostr.GetPublicKey(sk)
	if err != nil {
		return err
	}

	sharedKey, err := nip04.ComputeSharedSecret(receiverPub, sk)
	if err != nil {
		return fmt.Errorf("invalid receiver public key: %s", receiverPub)
	}

	content, err := nip04.Encrypt(msg, sharedKey)
	if err != nil {
		return err
	}

	ev := nostr.Event{
		PubKey:    senderPub,
		CreatedAt: time.Now(),
		Kind:      4, // 4 indicates encrypted direct message
		Tags: nostr.Tags{
			nostr.Tag{
				"p", receiverPub, // tag the receiver
			},
		},
		Content: content,
	}

	err = ev.Sign(sk)
	if err != nil {
		return err
	}

	return c.Publish(ctx, ev)
}
