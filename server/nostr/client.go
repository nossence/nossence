package nostr

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip04"
	"github.com/nbd-wtf/go-nostr/nip19"
)

var logger = log.New("module", "nostr")

type Client struct {
	Relays map[string]*nostr.Relay
}

type IClient interface {
	Subscribe(ctx context.Context, filters []nostr.Filter) <-chan nostr.Event
	Repost(ctx context.Context, sk, id, author, raw string) error
	Mention(ctx context.Context, sk, msg string, mentions []string) error
	Metadata(ctx context.Context, sk, name, about, picture, nip05 string) error
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

func NewClient(ctx context.Context, uris []string) (*Client, error) {
	rs := map[string]*nostr.Relay{}
	for _, uri := range uris {
		r, err := nostr.RelayConnect(ctx, uri)
		if err != nil {
			logger.Warn("failed to connect to relay, skipping...", "uri", uri, "err", err)
			continue
		}
		rs[uri] = r
	}

	return &Client{
		Relays: rs,
	}, nil
}

func (c *Client) Subscribe(ctx context.Context, filters []nostr.Filter) <-chan nostr.Event {
	ch := make(chan nostr.Event)
	for uri, r := range c.Relays {
		logger.Info("subscribing to relay", "uri", uri)
		sub := r.Subscribe(ctx, filters)

		// FIXME: fragile, need to refactor
		go func(uri string, relay *nostr.Relay, subscription *nostr.Subscription) {
			for {
				select {
				case ev := <-subscription.Events:
					if ev == nil {
						logger.Debug("received nil event, channel may closed", "uri", uri)
					} else {
						ch <- *ev
					}
				case notice := <-relay.Notices:
					logger.Warn("relay notice", "uri", uri, "notice", notice)
				case <-relay.ConnectionContext.Done():
					err := relay.ConnectionError
					logger.Error("relay connection error, try to reconnect", "uri", uri, "err", err)

					// try to reconnect for at most 5 times
					reconnected := c.reconnect(relay, 5)
					if !reconnected {
						logger.Error("failed to reconnect to relay, closing channel", "uri", uri)
						return
					}

					subscription = relay.Subscribe(ctx, filters)
					logger.Info("reconnected to relay", "uri", uri)
				}
			}
		}(uri, r, sub)
	}

	return ch
}

// Publish a signed event to all relays
func (c *Client) Publish(ctx context.Context, ev nostr.Event) error {
	for uri, r := range c.Relays {
		status, err := r.Publish(ctx, ev)
		if err != nil {
			logger.Debug("failed to publish event to relay, try to reconnect and resend", "uri", uri, "id", ev.ID, "err", err)
			c.reconnect(r, 1)
			status, err = r.Publish(ctx, ev)
		}
		switch status {
		case nostr.PublishStatusSucceeded:
			logger.Debug("published event to relay", "uri", uri, "id", ev.ID)
		case nostr.PublishStatusFailed:
			logger.Error("failed to publish event to relay, skip", "uri", uri, "id", ev.ID, "err", err)
		case nostr.PublishStatusSent:
			logger.Warn("event may or may not published to relay", "uri", uri, "id", ev.ID, "err", err)
		}
	}
	return nil
}

func (c *Client) reconnect(relay *nostr.Relay, retries int) bool {
	for i := 0; i < retries; i++ {
		err := relay.Connect(context.Background())
		if err != nil {
			logger.Debug("failed to reconnect to relay, retrying...", "uri", relay.URL, "err", err)
			time.Sleep(30 * time.Second)
			continue
		}
		return true
	}
	return false
}

// Repost an event
func (c *Client) Repost(ctx context.Context, sk, eventID, authorPub, raw string) error {
	note, _ := nip19.EncodeNote(eventID)
	logger.Debug("reposting event", "event_id", eventID, "note", note, "author_pub", authorPub, "raw", raw)
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
		// To align with repost requirement on Damus, there's needs
		// to set the raw origin event in content field
		Content:   raw,
		CreatedAt: time.Now(),
	}

	err = ev.Sign(sk)
	if err != nil {
		return err
	}

	return c.Publish(ctx, ev)
}

func (c *Client) Mention(ctx context.Context, sk, msg string, mentions []string) error {
	senderPub, err := nostr.GetPublicKey(sk)
	if err != nil {
		return err
	}

	if err != nil {
		return err
	}

	mentionTags := nostr.Tags{}
	for _, m := range mentions {
		mentionTags = append(mentionTags, nostr.Tag{
			"p", m, "", "mention",
		})
	}

	ev := nostr.Event{
		PubKey:    senderPub,
		CreatedAt: time.Now(),
		Kind:      1,
		Tags:      mentionTags,
		Content:   msg,
	}

	err = ev.Sign(sk)
	if err != nil {
		return err
	}

	return c.Publish(ctx, ev)
}

func (c *Client) Metadata(ctx context.Context, sk, name, about, picture, nip05 string) error {
	senderPub, err := nostr.GetPublicKey(sk)
	if err != nil {
		return err
	}

	content := map[string]string{
		"name":         name,
		"username":     name,
		"display_name": name,
		"about":        about,
		"picture":      picture,
	}
	if nip05 != "" {
		content["nip05"] = nip05
	}
	contentJson, err := json.Marshal(content)
	if err != nil {
		return err
	}

	ev := nostr.Event{
		PubKey:    senderPub,
		CreatedAt: time.Now(),
		Kind:      0,
		Content:   string(contentJson),
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
