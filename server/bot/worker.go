package bot

import (
	"context"
	"runtime"
	"sync"
	"time"

	n "github.com/dyng/nosdaily/nostr"
	"github.com/dyng/nosdaily/service"
	"github.com/dyng/nosdaily/types"
	"github.com/nbd-wtf/go-nostr"
)

type Worker struct {
	config  *types.Config
	client  n.IClient
	service service.IService
}

var (
	QuoteComment = "Here are the Top %d events curated for You"
)

func NewWorker(ctx context.Context, client n.IClient, service service.IService, config *types.Config) (*Worker, error) {
	return &Worker{
		config:  config,
		client:  client,
		service: service,
	}, nil
}

func (w *Worker) Run(ctx context.Context) error {
	limit := 10
	skip := 0
	hasNext := true
	var err error

	err = w.UpdateMain(ctx)
	if err != nil {
		logger.Error("error occurs in main update", "err", err)
	}

	for hasNext {
		hasNext, err = w.Batch(ctx, limit, skip)
		if err != nil {
			logger.Error("error occurs during batch execution", "err", err)
		}
		skip += limit
	}

	logger.Info("run finished")
	return nil
}

func (w *Worker) UpdateMain(ctx context.Context) error {
	logger.Info("updating main channel")
	mainSK := w.config.Bot.SK
	return w.Push(ctx, "", mainSK, PushInterval, PushSize, true)
}

func (w *Worker) Batch(ctx context.Context, limit, skip int) (hasNext bool, err error) {
	logger.Info("running batch", "limit", limit, "skip", skip)
	subscribers, err := w.service.ListSubscribers(ctx, limit, skip)
	if err != nil {
		return false, err
	}

	subChan := make(chan *types.Subscriber)
	var wg sync.WaitGroup

	for i := 1; i <= runtime.NumCPU(); i++ {
		wg.Add(1)
		go w.pusher(i, ctx, subChan, &wg)
	}

	for _, subscriber := range subscribers {
		sub := subscriber
		logger.Debug("dispatch subscriber", "pubkey", sub.Pubkey)
		subChan <- &sub
	}
	close(subChan)

	wg.Wait()
	logger.Info("batch push finished", "limit", limit, "skip", skip)

	return len(subscribers) >= limit, nil
}

func (w *Worker) Push(ctx context.Context, subscriberPub, channelSK string, timeRange time.Duration, limit int, useRepost bool) error {
	start := time.Now().Add(-1 * timeRange)
	end := time.Now()
	logger.Debug("start to repost feed", "userPub", subscriberPub, "start", start, "end", end, "limit", limit)
	feed := w.service.GetFeed(subscriberPub, start, end, limit)
	if len(feed) == 0 {
		logger.Warn("got empty feed", "subscriberPub", subscriberPub)
		return nil
	}
	logger.Debug("got feed", "subscriberPub", subscriberPub, "size", len(feed))

	channelPub, _ := nostr.GetPublicKey(channelSK)
	var eventIds []string
	for _, post := range feed {
		eventIds = append(eventIds, post.Id)
	}

	if useRepost {
		for _, post := range feed {
			err := w.client.Repost(ctx, channelSK, post.Id, post.Pubkey, post.Raw)
			if err != nil {
				logger.Warn("failed to repost event", "channelPub", channelPub, "id", post.Id, "err", err)
			}
		}
	} else {
		for _, post := range feed {
			err := w.client.Quote(ctx, channelSK, "", []string{post.Id})
			if err != nil {
				logger.Warn("failed to quote event", "channelPub", channelPub, "eventIds", eventIds, "err", err)
			}
		}
	}

	logger.Info("reposted feed", "subscriberPub", subscriberPub, "channelPub", channelPub, "eventIds", eventIds, "useRepost", useRepost)
	return nil
}

func (w *Worker) pusher(id int, ctx context.Context, subscribers <-chan *types.Subscriber, wg *sync.WaitGroup) {
	defer wg.Done()
	for subscriber := range subscribers {
		logger.Debug("push to subscriber", "pubkey", subscriber.Pubkey, "pusher", id)
		if subscriber.UnsubscribedAt != nil {
			logger.Info("skipping non subscriber", "pubkey", subscriber.Pubkey)
			continue
		}
		err := w.Push(ctx, subscriber.Pubkey, subscriber.ChannelSecret, PushInterval, PushSize, false)
		if err != nil {
			logger.Warn("failed to run worker for subscriber", "pubkey", subscriber.Pubkey, "err", err)
		} else {
			logger.Info("worker finished for subscriber", "pubkey", subscriber.Pubkey)
		}
	}
}
