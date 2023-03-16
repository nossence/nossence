package service

import (
	"context"
	"time"

	"github.com/dyng/nosdaily/database"
	"github.com/dyng/nosdaily/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/nbd-wtf/go-nostr"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type Service struct {
	config *types.Config
	neo4j  *database.Neo4jDb
}

type IService interface {
	GetFeed() any
}

func NewService(config *types.Config, neo4j *database.Neo4jDb) *Service {
	return &Service{
		config: config,
		neo4j:  neo4j,
	}
}

type FeedEntry struct {
	Id        string    `json:"event_id"`
	Kind      int       `json:"kind"`
	Pubkey    string    `json:"pubkey"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	Summary   string    `json:"summary"`
	Title     string    `json:"title"`
	Image     string    `json:"image"`
	Like      int       `json:"like"`
	Repost    int       `json:"repost"`
	Reply     int       `json:"reply"`
	Zap       int       `json:"zap"`
	Relay     []string  `json:"relay"`
}

func (s *Service) GetFeed() any {
	posts, err := s.neo4j.ExecuteRead(func(tx neo4j.ManagedTransaction) (any, error) {
		ctx := context.Background()

		result, err := tx.Run(ctx, "match (p:Post) return p.id, p.kind, p.author, p.content, p.created_at;", nil)
		if err != nil {
			return nil, err
		}

		posts := make([]FeedEntry, 0)
		for result.Next(ctx) {
			record := result.Record()
			post := FeedEntry{
				Id:        record.Values[0].(string),
				Kind:      int(record.Values[1].(int64)),
				Pubkey:    record.Values[2].(string),
				Content:   record.Values[3].(string),
				CreatedAt: time.Unix(record.Values[4].(int64), 0),
			}
			posts = append(posts, post)
		}
		return posts, nil
	})

	if err != nil {
		log.Error("Failed to get feed", "err", err)
		return nil
	} else {
		return posts
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
		log.Warn("Unsupported event kind", "kind", event.Kind)
		return nil
	}
}

func (s *Service) StorePost(post types.Post) error {
	log.Debug("Storing post", "id", post.Id, "kind", post.Kind, "author", post.Author, "created_at", post.CreatedAt)
	_, err := s.neo4j.ExecuteWrite(func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(context.Background(), "create (p:Post {id: $Id, kind: $Kind, author: $Author, content: $Content, created_at: $CreatedAt});",
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

func (s *Service) CreateSubscriber(pubkey, channelSK string, subscribedAt time.Time) error {
	log.Debug("Create subscriber", "pubkey", pubkey)
	_, err := s.neo4j.ExecuteWrite(func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(context.Background(), "MERGE (s:Subscriber {pubkey: $Pubkey}) ON CREATE SET s.channel_secret = $ChannelSecret, s.subscribed_at = $SubscribedAt, s.unsubscribed_at = null;",
			map[string]any{
				"Pubkey":        pubkey,
				"ChannelSecret": channelSK,
				"SubscribedAt":  subscribedAt.Unix(),
			})
		return nil, err
	})
	return err
}

func (s *Service) GetSubscriber(pubkey string) *types.Subscriber {
	subscriber, err := s.neo4j.ExecuteRead(func(tx neo4j.ManagedTransaction) (any, error) {
		ctx := context.Background()

		result, err := tx.Run(ctx, "MATCH (s:Subscriber {pubkey: $Pubkey}) RETURN s.pubkey, s.channel_secret, s.subscribed_at, s.unsubscribed_at;",
			map[string]any{
				"Pubkey": pubkey,
			})
		if err != nil {
			return nil, err
		}

		record, err := result.Single(ctx)
		if err != nil {
			return nil, err
		}

		subscriber := types.Subscriber{
			Pubkey:         record.Values[0].(string),
			ChannelSecret:  record.Values[1].(string),
			SubscribedAt:   time.Unix(record.Values[2].(int64), 0),
			UnsubscribedAt: time.Unix(record.Values[3].(int64), 0),
		}

		return subscriber, nil
	})

	if err != nil {
		log.Error("Failed to get subscriber", "err", err)
		return nil
	}

	if result, ok := subscriber.(types.Subscriber); ok {
		return &result
	}

	return nil
}

func (s *Service) DeleteSubscriber(pubkey string, unsubscribedAt time.Time) error {
	log.Debug("Deleting subscriber", "pubkey", pubkey)
	_, err := s.neo4j.ExecuteWrite(func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(context.Background(), "MATCH (s:Subscriber {pubkey: $Pubkey}) SET s.unsubscribed_at = $UnsubscribedAt;",
			map[string]any{
				"Pubkey":         pubkey,
				"UnsubscribedAt": unsubscribedAt.Unix(),
			})
		return nil, err
	})
	return err
}
