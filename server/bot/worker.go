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

func (w *Worker) Run(ctx context.Context, userPub, subSK string, timeRange time.Duration, limit int) error {
	_ = timeRange
	_ = limit
	feed := w.service.GetFeed() // TODO: should be GetFeed(userPub, timeRange, limit)
	if feed == nil {
		logger.Warn("got empty feed", "userPub", userPub)
		return nil
	}

	var eventIds []string
	if posts, ok := feed.([]service.FeedEntry); ok {
		for _, post := range posts {
			err := w.client.Repost(ctx, subSK, post.Id, post.Pubkey)
			if err != nil {
				logger.Warn("failed to repost event", "id", post.Id, "err", err)
			}
			eventIds = append(eventIds, post.Id)
		}
	}

	logger.Info("reposted feed", "userPub", userPub, "eventIds", eventIds)
	return nil
}
