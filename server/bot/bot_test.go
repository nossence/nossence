package bot

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	n "github.com/dyng/nosdaily/nostr"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip04"
	"github.com/stretchr/testify/assert"
)

var botSK string
var userSK string

func getRelayURIs() []string {
	return strings.Split(os.Getenv("NOSTR_RELAY_URI"), ",")
}

func TestMain(m *testing.M) {
	botSK = nostr.GeneratePrivateKey()
	userSK = nostr.GeneratePrivateKey()

	code := m.Run()
	os.Exit(code)
}

func TestNewBot(t *testing.T) {
	client, err := n.NewClient(context.Background(), getRelayURIs())
	assert.NoError(t, err)

	bot, err := NewBot(context.Background(), client, botSK)
	assert.NoError(t, err)
	assert.NotNil(t, bot)
}

// bot should listen to user's subscribe post with mention
func TestListen(t *testing.T) {
	client, err := n.NewClient(context.Background(), getRelayURIs())
	assert.NoError(t, err)

	bot, err := NewBot(context.Background(), client, botSK)
	assert.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c, err := bot.Listen(ctx)
	assert.NoError(t, err)

	botPub, err := nostr.GetPublicKey(botSK)
	assert.NoError(t, err)
	userPub, err := nostr.GetPublicKey(userSK)
	assert.NoError(t, err)
	ev := nostr.Event{
		Content:   "#[0] #subscribe",
		CreatedAt: time.Now(),
		Kind:      1,
		PubKey:    userPub,
		Tags: nostr.Tags{
			nostr.Tag{"p", botPub, "", "mention"},
		},
	}
	ev.Sign(userSK)

	err = client.Publish(context.Background(), ev)
	assert.NoError(t, err)

	msg := <-c
	assert.NotNil(t, msg)
	t.Logf("msg: %v", msg)
}

// bot should generate a sub account and store it with a reference to user
func TestGenerateSubAccount(t *testing.T) {
	t.Skip("not implemented yet")
}

// bot should send a welcome message to user mentioning the sub account
func TestSendWelcomeMessage(t *testing.T) {
	subSK := nostr.GeneratePrivateKey()
	userPub, err := nostr.GetPublicKey(userSK)
	assert.NoError(t, err)

	client, err := n.NewClient(context.Background(), getRelayURIs())
	assert.NoError(t, err)

	bot, err := NewBot(context.Background(), client, botSK)
	assert.NoError(t, err)

	c := client.Subscribe(context.Background(), []nostr.Filter{
		{Kinds: []int{4}, Tags: nostr.TagMap{"p": []string{userPub}}},
	})
	bot.SendWelcomeMessage(context.Background(), subSK, userPub)

	ev := <-c
	t.Logf("event: %v", ev)
	assert.NotNil(t, ev)

	sharedSK, err := nip04.ComputeSharedSecret(bot.pub, userSK)
	assert.NoError(t, err)
	msg, err := nip04.Decrypt(ev.Content, sharedSK)
	assert.NoError(t, err)
	t.Logf("decrypted msg: %v", msg)
}
