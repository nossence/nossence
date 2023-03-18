package nostr

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/stretchr/testify/assert"
)

func getRelayURIs() []string {
	return []string{"ws://localhost:8090"}
}

func getIdentity() (sk, pub string) {
	sk = nostr.GeneratePrivateKey()
	pub, _ = nostr.GetPublicKey(sk)
	return
}

func getReceiverPub() string {
	receiverPub := os.Getenv("NOSTR_TEST_RECEIVER_PUB")
	pub, err := DecodeNpub(receiverPub)
	if err != nil {
		return ""
	}

	return pub
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
	c := client.Subscribe(ctx, filters)

	defer cancel()

	ev := <-c
	t.Logf("event: %v", ev)
	assert.NotNil(t, ev)
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
	receiverPub := getReceiverPub()
	err = client.SendMessage(context.Background(), sk, receiverPub, msg)
	assert.NoError(t, err)
}

func TestRepost(t *testing.T) {
	client, err := NewClient(context.Background(), getRelayURIs())
	assert.NoError(t, err)

	sk, _ := getIdentity()
	assert.NoError(t, err)
	eventID := "foo"
	authorPub := "bar"
	raw := ""
	err = client.Repost(context.Background(), sk, eventID, authorPub, raw)
	assert.NoError(t, err)
}
