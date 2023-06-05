package bot

import (
	"context"
	"testing"
	"time"

	"github.com/dyng/nosdaily/nostr"
	"github.com/dyng/nosdaily/service"
	"github.com/dyng/nosdaily/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestWorkerRun(t *testing.T) {
	mockClient := new(nostr.MockClient)
	mockClient.On("Repost", context.Background(), "channel_secret", "event_id", "author_pub", "raw_event").Return(nil)

	mockService := new(service.MockService)
	mockService.On("GetFeed").Return([]types.FeedEntry{
		{
			Id:     "event_id",
			Pubkey: "author_pub",
			Raw:    "raw_event",
		},
	})

	worker, err := NewWorker(context.Background(), mockClient, mockService, nil)
	assert.NoError(t, err)

	worker.Push(context.Background(), "subscriber_pub", "channel_secret", time.Hour, 10, false)
	mockService.AssertCalled(t, "GetFeed", "subscriber_pub", mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time"), 10)
	mockClient.AssertCalled(t, "Repost", context.Background(), "channel_secret", "event_id", "author_pub", "raw_event")
}
