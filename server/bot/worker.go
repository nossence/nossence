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
	start := time.Now().Add(-time.Hour)
	end := time.Now()
	logger.Debug("start to repost feed", "userPub", userPub, "start", start, "end", end, "limit", limit)
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
