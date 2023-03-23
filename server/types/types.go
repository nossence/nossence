package types

import "time"

type Subscriber struct {
	Pubkey         string
	ChannelSecret  string
	SubscribedAt   *time.Time
	UnsubscribedAt *time.Time
}

type FeedEntry struct {
	Id        string    `json:"event_id"`
	Kind      int       `json:"kind"`
	Pubkey    string    `json:"pubkey"`
	CreatedAt time.Time `json:"created_at"`
	Score     float64   `json:"score"`
	Raw       string    `json:"raw"`
}
