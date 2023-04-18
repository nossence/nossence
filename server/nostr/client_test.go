package nostr

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/stretchr/testify/assert"
)

var relays = []string{"wss://relay.damus.io"}

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
	_, err := NewClient(context.Background(), relays)
	assert.NoError(t, err)
}

func TestSubscribe(t *testing.T) {
	client, err := NewClient(context.Background(), relays)
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
	client, err := NewClient(context.Background(), relays)
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
	client, err := NewClient(context.Background(), relays)
	assert.NoError(t, err)

	sk, _ := getIdentity()
	msg := "foo"
	receiverPub := getReceiverPub()
	err = client.SendMessage(context.Background(), sk, receiverPub, msg)
	assert.NoError(t, err)
}

func TestRepost(t *testing.T) {
	client, err := NewClient(context.Background(), relays)
	assert.NoError(t, err)

	sk, _ := getIdentity()
	pk, _ := nostr.GetPublicKey(sk)
	npub, _ := nip19.EncodePublicKey(pk)
	fmt.Printf("sk: %s, pub: %s, npub: %s", sk, pk, npub)
	assert.NoError(t, err)
	eventID := "c8436ce1b543ae7c9cabe2da4666cf566410c36d48886d732d2e19165130c652"
	authorPub := "aba7339fe76595d4ad5bff333f1ba1e9198907588a49df4519a3ade60cc1f998"
	raw := "{\"pubkey\":\"aba7339fe76595d4ad5bff333f1ba1e9198907588a49df4519a3ade60cc1f998\",\"content\":\"坚持，但不要执念。\\n\\npersevere, but don't obsess.\",\"id\":\"c8436ce1b543ae7c9cabe2da4666cf566410c36d48886d732d2e19165130c652\",\"created_at\":1677890182,\"sig\":\"4db2f023ddce2c9386325770f13a80e0470f20fd4df5535bd536c501377c29e0e80e47f88b5a0077ea9782d20746ce9ee48afeeb9043cdc6266ffdd492485433\",\"kind\":1,\"tags\":[]}"
	err = client.Repost(context.Background(), sk, eventID, authorPub, raw)
	assert.Error(t, err)
}
