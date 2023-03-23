package nostr

import (
	"context"

	"github.com/nbd-wtf/go-nostr"
	"github.com/stretchr/testify/mock"
)

type MockClient struct {
	mock.Mock
}

// Metadata implements IClient
func (m *MockClient) Metadata(ctx context.Context, sk string, name string, about string, picture string, nip05 string) error {
	args := m.Called(ctx, sk, name, about, picture, nip05)
	return args.Error(0)
}

func (m *MockClient) Subscribe(ctx context.Context, filters []nostr.Filter) <-chan nostr.Event {
	args := m.Called(ctx, filters)
	return args.Get(0).(<-chan nostr.Event)
}

func (m *MockClient) Repost(ctx context.Context, sk, id, author, raw string) error {
	args := m.Called(ctx, sk, id, author, raw)
	return args.Error(0)
}

func (m *MockClient) Mention(ctx context.Context, sk, msg string, mentions []string) error {
	args := m.Called(ctx, sk, msg, mentions)
	return args.Error(0)
}
