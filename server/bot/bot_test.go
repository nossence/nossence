package bot

import (
	"context"
	"testing"

	n "github.com/dyng/nosdaily/nostr"
	"github.com/dyng/nosdaily/service"
	"github.com/dyng/nosdaily/types"
	"github.com/nbd-wtf/go-nostr"
	"github.com/stretchr/testify/assert"
)

var botSK = nostr.GeneratePrivateKey()
var subscriberSK = nostr.GeneratePrivateKey()
var relays = []string{"ws://localhost:8090"}
var config = &types.Config{
	Bot: types.BotConfig{
		Relays: relays,
		SK:     botSK,
		Metadata: types.MetadataConfig{
			Name:    "nossence",
			About:   "a recommender engine for nostr",
			ChannelName: "nossence for %s",
			ChannelAbout: "nossence curated content for %s",
			Picture: "",
		},
	},
}

func TestNewBot(t *testing.T) {
	mockClient := new(n.MockClient)
	mockService := new(service.MockService)

	bot, err := NewBot(context.Background(), mockClient, mockService, config)
	assert.NoError(t, err)
	assert.NotNil(t, bot)
}

// bot should listen to subscribers' mention
func TestListen(t *testing.T) {
	mockClient := new(n.MockClient)
	mockService := new(service.MockService)

	bot, err := NewBot(context.Background(), mockClient, mockService, config)
	assert.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c, err := bot.Listen(ctx)
	assert.NoError(t, err)

	// botPub, err := nostr.GetPublicKey(botSK)
	// assert.NoError(t, err)
	// subscriberPub, err := nostr.GetPublicKey(subscriberSK)
	// assert.NoError(t, err)
	// ev := nostr.Event{
	// 	Content:   "#[0] #subscribe",
	// 	CreatedAt: time.Now(),
	// 	Kind:      1,
	// 	PubKey:    subscriberPub,
	// 	Tags: nostr.Tags{
	// 		nostr.Tag{"p", botPub, "", "mention"},
	// 	},
	// }
	// ev.Sign(subscriberSK)

	// err = mockClient.Publish(context.Background(), ev)
	// assert.NoError(t, err)

	msg := <-c
	assert.NotNil(t, msg)
	t.Logf("msg: %v", msg)
}

// bot should create a channel and store it with a reference to subscriber
func TestGetOrCreateSubscription(t *testing.T) {
	mockClient := new(n.MockClient)
	mockService := new(service.MockService)

	bot, err := NewBot(context.Background(), mockClient, mockService, config)
	assert.NoError(t, err)

	subscriberPub, err := nostr.GetPublicKey(subscriberSK)
	assert.NoError(t, err)

	channelSK, created, err := bot.GetOrCreateSubscription(context.Background(), subscriberPub)
	assert.NoError(t, err)
	assert.True(t, created)
	assert.NotNil(t, channelSK)

	channelSK, created, err = bot.GetOrCreateSubscription(context.Background(), subscriberPub)
	assert.NoError(t, err)
	assert.False(t, created)
	assert.NotNil(t, channelSK)
}

// bot should send a welcome message to subscriber mentioning the channel
func TestSendWelcomeMessage(t *testing.T) {
	mockClient := new(n.MockClient)
	mockService := new(service.MockService)

	channelSK := nostr.GeneratePrivateKey()
	subscriberPub, err := nostr.GetPublicKey(subscriberSK)
	assert.NoError(t, err)

	bot, err := NewBot(context.Background(), mockClient, mockService, config)
	assert.NoError(t, err)

	// c := client.Subscribe(context.Background(), []nostr.Filter{
	// 	{Kinds: []int{4}, Tags: nostr.TagMap{"p": []string{subscriberPub}}},
	// })
	bot.SendWelcomeMessage(context.Background(), channelSK, subscriberPub)

	// ev := <-c
	// t.Logf("event: %v", ev)
	// assert.NotNil(t, ev)
	// TODO: should check welcome message mentions the right person
}
