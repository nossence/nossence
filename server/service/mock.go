package service

import (
	"context"
	"time"

	"github.com/dyng/nosdaily/types"
	"github.com/stretchr/testify/mock"
)

type MockService struct {
	mock.Mock
}

func (m *MockService) GetFeed(subscriberPub string, start time.Time, end time.Time, limit int) []types.FeedEntry {
	args := m.Called(subscriberPub, start, end, limit)
	return args.Get(0).([]types.FeedEntry)
}

func (m *MockService) ListSubscribers(ctx context.Context, limit, skip int) ([]types.Subscriber, error) {
	args := m.Called(ctx, limit, skip)
	return args.Get(0).([]types.Subscriber), args.Error(1)
}

func (m *MockService) GetSubscriber(pubkey string) *types.Subscriber {
	args := m.Called(pubkey)
	return args.Get(0).(*types.Subscriber)
}

func (m *MockService) CreateSubscriber(pubkey, channelSK string, subscribedAt time.Time) error {
	args := m.Called(pubkey, channelSK, subscribedAt)
	return args.Error(0)
}

func (m *MockService) DeleteSubscriber(pubkey string, unsubscribedAt time.Time) error {
	args := m.Called(pubkey, unsubscribedAt)
	return args.Error(0)
}

func (m *MockService) RestoreSubscriber(pubkey string, subscribedAt time.Time) (bool, error) {
	args := m.Called(pubkey, subscribedAt)
	return args.Bool(0), args.Error(1)
}
