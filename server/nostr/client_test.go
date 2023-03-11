package nostr

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/stretchr/testify/assert"
)

func getRelayURIs() []string {
	return strings.Split(os.Getenv("NOSTR_RELAY_URI"), ",")
}

func getIdentity() (sk, pub string) {
	myPrivateKey := os.Getenv("NOSTR_PRIVATE_KEY")

	_, val, err := nip19.Decode(myPrivateKey)
	if err != nil {
		return "", ""
	}
	if sk, ok := val.(string); ok {
		pub, err := nostr.GetPublicKey(sk)
		if err != nil {
			return "", ""
		}

		return sk, pub
	}

	return "", ""
}

func TestNewClient(t *testing.T) {
	_, err := NewClient(context.Background(), getRelayURIs())
	assert.NoError(t, err)
}

func TestSubscribe(t *testing.T) {
	client, err := NewClient(context.Background(), getRelayURIs())
	assert.NoError(t, err)

	until := time.Now()
	filters := []nostr.Filter{{
		Kinds: []int{1},
		Until: &until,
		Limit: 10,
	}}

	ctx, cancel := context.WithCancel(context.Background())
	sub := client.Relays[0].Subscribe(ctx, filters)

	go func() {
		time.Sleep(1 * time.Second)
		cancel()
	}()

	events := []*nostr.Event{}
	for ev := range sub.Events {
		t.Logf("event: %+v", ev)
		events = append(events, ev)
	}

	assert.GreaterOrEqual(t, len(events), 1)
}

func TestPublish(t *testing.T) {
	client, err := NewClient(context.Background(), getRelayURIs())
	assert.NoError(t, err)

	sk, pub := getIdentity()
	ev := nostr.Event{
		PubKey:    pub,
		CreatedAt: time.Now(),
		Kind:      1,
		Tags:      nil,
		Content:   "Hello World!",
	}
	err = ev.Sign(sk)
	assert.NoError(t, err)

	err = client.Publish(context.Background(), ev)
	assert.NoError(t, err)
}

func TestSendMessage(t *testing.T) {
	client, err := NewClient(context.Background(), getRelayURIs())
	assert.NoError(t, err)

	sk, _ := getIdentity()
	msg := "foo"
	receiverPub := ""
	err = client.SendMessage(context.Background(), sk, receiverPub, msg)
	assert.NoError(t, err)
}

func TestRepost(t *testing.T) {
	client, err := NewClient(context.Background(), getRelayURIs())
	assert.NoError(t, err)

	sk, pub := getIdentity()
	assert.NoError(t, err)
	eventID := "db3daf21b32bc40beec979343d8a139175c14e62f2e9c7e84528b24dc79e5349"
	authorPub := pub
	err = client.Repost(context.Background(), sk, eventID, authorPub)
	assert.NoError(t, err)
}
