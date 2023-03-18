package bot

import (
	"context"
	"time"

	n "github.com/dyng/nosdaily/nostr"
	"github.com/dyng/nosdaily/service"
)

type Worker struct {
	client  n.IClient
	service service.IService
}

func NewWorker(ctx context.Context, client n.IClient, service service.IService) (*Worker, error) {
	return &Worker{
		client:  client,
		service: service,
	}, nil
}

func (w *Worker) Batch(ctx context.Context, limit, skip int) (bool, error) {
	logger.Info("running batch", "limit", limit, "skip", skip)
	subscribers, err := w.service.ListSubscribers(ctx, limit, skip)
	if err != nil {
		return false, err
	}

	for _, subscriber := range subscribers {
		if subscriber.UnsubscribedAt != nil {
			logger.Info("skipping unsubscribed user", "pubkey", subscriber.Pubkey)
			continue
		}
		err = w.Run(ctx, subscriber.Pubkey, subscriber.ChannelSecret, time.Hour, 10)
		if err != nil {
			logger.Warn("failed to run worker for user", "pubkey", subscriber.Pubkey, "err", err)
		}
	}

	return false, nil
}

func (w *Worker) Run(ctx context.Context, userPub, subSK string, timeRange time.Duration, limit int) error {
	_ = timeRange
	_ = limit
	start := time.Now().Add(-time.Hour)
	end := time.Now()
	feed := w.service.GetFeed(userPub, start, end, limit)
	if len(feed) == 0 {
		logger.Warn("got empty feed", "userPub", userPub)
		return nil
	}

	var eventIds []string
	for _, post := range feed {
		err := w.client.Repost(ctx, subSK, post.Id, post.Pubkey)
		if err != nil {
			logger.Warn("failed to repost event", "id", post.Id, "err", err)
		}
		eventIds = append(eventIds, post.Id)
	}

	logger.Info("reposted feed", "userPub", userPub, "eventIds", eventIds)
	return nil
}
