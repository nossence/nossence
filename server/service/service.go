package service

import (
	"context"
	"log"
	"time"

	"github.com/dyng/nosdaily/database"
	"github.com/dyng/nosdaily/types"
	"github.com/nbd-wtf/go-nostr"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type Service struct {
	neo4j *database.Neo4jDb
}

func NewService(neo4j *database.Neo4jDb) *Service {
	return &Service{
		neo4j: neo4j,
	}
}

func (s *Service) GetFeed() []types.Post {
	posts, err := s.neo4j.ExecuteRead(func(tx neo4j.ManagedTransaction) (any, error) {
		ctx := context.Background()

		result, err := tx.Run(ctx, "match (p:Post) return p.id, p.kind, p.author, p.content, p.created_at;", nil)
		if err != nil {
			return nil, err
		}

		posts := make([]types.Post, 0)
		for result.Next(ctx) {
			record := result.Record()
			post := types.Post{
				Id: record.Values[0].(string),
				Kind: int(record.Values[1].(int64)),
				// Author: record.Values[2].(string),
				Content: record.Values[3].(string),
				CreatedAt: time.Unix(record.Values[4].(int64), 0),
			}
			posts = append(posts, post)
		}
		return posts, nil
	})

	if err != nil {
		log.Printf("Error getting feed: %v\n", err)
		return nil
	} else {
		log.Printf("Get feed: %v\n", posts)
		return posts.([]types.Post)
	}
}

func (s *Service) StoreEvent(event *nostr.Event) error {
	switch event.Kind {
	case 1, 30023:
		post := types.Post{
			Id:        event.ID,
			Kind:      event.Kind,
			Author:    event.PubKey,
			Content:   event.Content,
			CreatedAt: event.CreatedAt,
		}
		return s.StorePost(post)
	default:
		// TODO: print warning
		return nil
	}
}

func (s *Service) StorePost(post types.Post) error {
	log.Printf("Store post: %v\n", post)
	_, err := s.neo4j.ExecuteWrite(func(tx neo4j.ManagedTransaction) (any, error) {
		_, err:= tx.Run(context.Background(), "create (p:Post {id: $Id, kind: $Kind, content: $Content, created_at: $CreatedAt});",
		map[string]any{
			"Id":        post.Id,
			"Kind":      post.Kind,
			"Author":    post.Author,
			"Content":   post.Content,
			"CreatedAt": post.CreatedAt.Unix(),
		})
		return nil, err
	})
	return err
}
