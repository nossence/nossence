package types

import "time"

type Post struct {
	Id        string
	Kind      int
	Author    string
	Content   string
	CreatedAt time.Time
}

type User struct {
	Pubkey string
}
