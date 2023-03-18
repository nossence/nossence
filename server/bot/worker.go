package bot

import (
	"context"
	"time"

	n "github.com/dyng/nosdaily/nostr"
	"github.com/dyng/nosdaily/service"
	"github.com/nbd-wtf/go-nostr"
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
			logger.Info("skipping non subscriber", "pubkey", subscriber.Pubkey)
			continue
		}
		err = w.Run(ctx, subscriber.Pubkey, subscriber.ChannelSecret, time.Hour, 10)
		if err != nil {
			logger.Warn("failed to run worker for subscriber", "pubkey", subscriber.Pubkey, "err", err)
		}
	}

	return false, nil
}

func (w *Worker) Run(ctx context.Context, subscriberPub, subSK string, timeRange time.Duration, limit int) error {
	_ = timeRange
	_ = limit
	start := time.Now().Add(-time.Hour)
	end := time.Now()
	feed := w.service.GetFeed(subscriberPub, start, end, limit)
	if len(feed) == 0 {
		logger.Warn("got empty feed", "subscriberPub", subscriberPub)
		return nil
	}

	var eventIds []string
	subPub, _ := nostr.GetPublicKey(subSK)
	for _, post := range feed {
		err := w.client.Repost(ctx, subSK, post.Id, post.Pubkey)
		if err != nil {
			logger.Warn("failed to repost event", "subPub", subPub, "id", post.Id, "err", err)
		}
		eventIds = append(eventIds, post.Id)
	}

	logger.Debug("reposted feed", "subscriberPub", subscriberPub, "subPub", subPub, "eventIds", eventIds)
	return nil
}
