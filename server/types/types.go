package types

import "time"

type Post struct {
	Id        string    `json:"id"`
	Kind      int       `json:"kind"`
	Author    string    `json:"author"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type User struct {
	Pubkey string `json:"pubkey"`
}

type Subscriber struct {
	Pubkey         string
	ChannelSecret  string
	SubscribedAt   time.Time
	UnsubscribedAt time.Time
}
