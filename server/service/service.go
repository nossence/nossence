package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/dyng/nosdaily/database"
	"github.com/dyng/nosdaily/types"
	algo "github.com/dyng/nossence-algo"
	"github.com/ethereum/go-ethereum/log"
	"github.com/go-co-op/gocron"
	"github.com/nbd-wtf/go-nostr"
	decodepay "github.com/nbd-wtf/ln-decodepay"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

var logger = log.New("module", "service")

type Service struct {
	config    *types.Config
	neo4j     *database.Neo4jDb
	engine    *algo.Engine
	scheduler *gocron.Scheduler
}

type IService interface {
	GetRecommendationsTrends(start time.Time, end time.Time, limit int) ([]nostr.Event, error)
	GetFeed(subscriberPub string, start time.Time, end time.Time, limit int) []types.FeedEntry
	ListSubscribers(ctx context.Context, limit, skip int) ([]types.Subscriber, error)
	GetSubscriber(pubkey string) *types.Subscriber
	CreateSubscriber(pubkey, channelSK string, subscribedAt time.Time) error
	DeleteSubscriber(pubkey string, unsubscribedAt time.Time) error
	RestoreSubscriber(pubkey string, subscribedAt time.Time) (bool, error)
}

func NewService(config *types.Config, neo4j *database.Neo4jDb) *Service {
	return &Service{
		config:    config,
		neo4j:     neo4j,
		scheduler: gocron.NewScheduler(time.UTC),
	}
}

func (s *Service) Init() error {
	_, err := s.neo4j.ExecuteWrite(func(tx neo4j.ManagedTransaction) (any, error) {
		ctx := context.Background()
		if _, err := tx.Run(ctx, "CREATE CONSTRAINT post_id_uniq IF NOT EXISTS FOR (p:Post) REQUIRE p.id IS UNIQUE;", nil); err != nil {
			return nil, err
		}
		if _, err := tx.Run(ctx, "CREATE CONSTRAINT user_pk_uniq IF NOT EXISTS FOR (u:User) REQUIRE u.pubkey IS UNIQUE;", nil); err != nil {
			return nil, err
		}
		return nil, nil
	})

	// init algo engine
	s.engine = algo.NewEngine(s.neo4j.GetDriver())

	// init cleanup task
	s.scheduler.Every(1).Day().At("00:00").Do(s.DailyJob)

	return err
}

func (s *Service) GetRecommendationsTrends(start time.Time, end time.Time, limit int) ([]nostr.Event, error) {
	// fetch trend feed by using an empty pubkey
	feed := s.GetFeed("", start, end, limit)

	events := make([]nostr.Event, 0, len(feed))
	for _, entry := range feed {
		var event nostr.Event
		err := json.Unmarshal([]byte(entry.Raw), &event)
		if err != nil {
			log.Error("Failed to parse raw event", "raw", entry.Raw, "err", err)
			continue
		}
		events = append(events, event)
	}
	return events, nil
}

func (s *Service) GetFeed(subscriberPub string, start time.Time, end time.Time, limit int) []types.FeedEntry {
	posts := s.engine.GetFeed(subscriberPub, start, end, limit)
	feed := make([]types.FeedEntry, 0, len(posts))
	for _, post := range posts {
		raw, err := s.readObject(post.Id)
		if err != nil {
			log.Error("Failed to read object", "id", post.Id, "err", err)
			continue
		}

		feed = append(feed, types.FeedEntry{
			Id:        post.Id,
			Kind:      post.Kind,
			Pubkey:    post.Pubkey,
			CreatedAt: post.CreatedAt,
			Score:     post.Score,
			Raw:       raw,
		})
	}
	return feed
}

func (s *Service) StoreEvent(event *nostr.Event) error {
	switch event.Kind {
	case 1:
		return s.StorePost(event)
	case 6:
		return s.StoreRepost(event)
	case 7:
		return s.StoreLike(event)
	case 3:
		return s.StoreContact(event)
	case 9735:
		return s.StoreZap(event)
	default:
		logger.Warn("Unsupported event kind", "kind", event.Kind)
		return nil
	}
}

func (s *Service) StorePost(event *nostr.Event) error {
	_, err := s.neo4j.ExecuteWrite(func(tx neo4j.ManagedTransaction) (any, error) {
		ctx := context.Background()

		// create user & post
		if err := s.saveUserAndPost(ctx, tx, event); err != nil {
			return nil, err
		}

		// create reply relation
		refs := event.Tags.GetAll([]string{"e"})
		if len(refs) > 0 {
			ref := refs[0]
			if _, err := tx.Run(ctx, "match (p:Post), (r:Post) where p.id = $Id and r.id = $RefId merge (p)-[:REPLY]->(r);",
				map[string]any{
					"Id":    event.ID,
					"RefId": ref.Value(),
				}); err != nil {
				return nil, err
			}
		}

		return nil, nil
	})

	return err
}

func (s *Service) StoreLike(event *nostr.Event) error {
	_, err := s.neo4j.ExecuteWrite(func(tx neo4j.ManagedTransaction) (any, error) {
		ctx := context.Background()

		// create user & post
		if err := s.saveUserAndPost(ctx, tx, event); err != nil {
			return nil, err
		}

		// create like relation
		refs := event.Tags.GetAll([]string{"e"})
		if len(refs) > 0 {
			ref := refs[0]
			if _, err := tx.Run(ctx, "match (p:Post), (r:Post) where p.id = $Id and r.id = $RefId merge (p)-[:LIKE]->(r);",
				map[string]any{
					"Id":    event.ID,
					"RefId": ref.Value(),
				}); err != nil {
				return nil, err
			}
		}

		return nil, nil
	})

	return err
}

func (s *Service) StoreRepost(event *nostr.Event) error {
	_, err := s.neo4j.ExecuteWrite(func(tx neo4j.ManagedTransaction) (any, error) {
		ctx := context.Background()

		// create user & post
		if err := s.saveUserAndPost(ctx, tx, event); err != nil {
			return nil, err
		}

		// create repost relation
		refs := event.Tags.GetAll([]string{"e"})
		if len(refs) > 0 {
			ref := refs[0]
			if _, err := tx.Run(ctx, "match (p:Post), (r:Post) where p.id = $Id and r.id = $RefId merge (p)-[:REPOST]->(r);",
				map[string]any{
					"Id":    event.ID,
					"RefId": ref.Value(),
				}); err != nil {
				return nil, err
			}
		}

		return nil, nil
	})

	return err
}

func (s *Service) StoreContact(event *nostr.Event) error {
	_, err := s.neo4j.ExecuteWrite(func(tx neo4j.ManagedTransaction) (any, error) {
		ctx := context.Background()

		// delete old follow relations
		if _, err := tx.Run(ctx, "match (u:User {pubkey: $Pubkey})-[r:FOLLOW]->() delete r;",
			map[string]any{
				"Pubkey": event.PubKey,
			}); err != nil {
			return nil, err
		}

		// create new follow relations
		tags := event.Tags.GetAll([]string{"p"})
		for _, pTag := range tags {
			if _, err := tx.Run(ctx, "merge (u:User {pubkey: $Pubkey}) merge (p:User {pubkey: $P}) merge (u)-[:FOLLOW]->(p);",
				map[string]any{
					"Pubkey": event.PubKey,
					"P":      pTag.Value(),
				}); err != nil {
				return nil, err
			}
		}

		return nil, nil
	})

	return err
}

func (s *Service) StoreZap(event *nostr.Event) error {
	// decode zap amount
	bolt11 := event.Tags.GetLast([]string{"bolt11"})
	invoice, err := decodepay.Decodepay(bolt11.Value())
	if err != nil {
		return err
	}
	amount := invoice.MSatoshi / 1000

	_, err = s.neo4j.ExecuteWrite(func(tx neo4j.ManagedTransaction) (any, error) {
		ctx := context.Background()

		// exit if not a zap to a post
		refs := event.Tags.GetAll([]string{"e"})
		if len(refs) == 0 {
			return nil, nil
		}

		// create user & post
		if err := s.saveUserAndPost(ctx, tx, event); err != nil {
			return nil, err
		}

		// create zap relation
		ref := refs[0]
		if _, err := tx.Run(ctx, "match (p:Post), (r:Post) where p.id = $Id and r.id = $RefId merge (p)-[:ZAP {amount: $Amount}]->(r);",
			map[string]any{
				"Id":     event.ID,
				"RefId":  ref.Value(),
				"Amount": amount,
			}); err != nil {
			return nil, err
		}

		return nil, nil
	})

	return err
}

func (s *Service) saveUserAndPost(ctx context.Context, tx neo4j.ManagedTransaction, event *nostr.Event) error {
	if _, err := tx.Run(ctx, "merge (u:User {pubkey: $Pubkey});",
		map[string]any{
			"Pubkey": event.PubKey,
		}); err != nil {
		return err
	}

	err := s.writeObject(event)
	if err != nil {
		log.Error("Failed to write object", "id", event.ID, "err", err)
		return err
	}

	if _, err := tx.Run(ctx, "merge (p:Post {id: $Id, kind: $Kind, author: $Author, created_at: $CreatedAt});",
		map[string]any{
			"Id":        event.ID,
			"Kind":      event.Kind,
			"Author":    event.PubKey,
			"CreatedAt": event.CreatedAt.Unix(),
		}); err != nil {
		return err
	}

	if _, err := tx.Run(ctx, "match (u:User), (p:Post) where u.pubkey = $Pubkey and p.id = $Id merge (u)-[:CREATE]->(p);",
		map[string]any{
			"Pubkey": event.PubKey,
			"Id":     event.ID,
		}); err != nil {
		return err
	}

	return nil
}

func (s *Service) DailyJob() {
	// clean old posts
	s.cleanPosts()

	// clean objects
	s.cleanObjs()

	// update user favorites
	s.updateFavorites()
}

func (s *Service) updateFavorites() {
	// timestamp of 2 days ago
	timestamp := time.Now().AddDate(0, 0, -2).Unix()

	_, err := s.neo4j.ExecuteWrite(func(tx neo4j.ManagedTransaction) (any, error) {
		ctx := context.Background()

		if _, err := tx.Run(ctx, "match (r:Post) where r.created_at > $Timestamp match (a:User)-[:CREATE]->(r:Post)-[:ZAP|REPLY|LIKE|REPOST]->(p:Post) merge (a)-[:LIKES]->(p)",
			map[string]any{
				"Timestamp": timestamp,
			}); err != nil {
			return nil, err
		}

		if _, err := tx.Run(ctx, "match (:User)-[s:SIMILAR]->(:User) delete s",
			map[string]any{}); err != nil {
			return nil, err
		}

		if _, err := tx.Run(ctx, "CALL gds.nodeSimilarity.write('myGraph', { similarityCutoff: 0.01, degreeCutoff: 3, writeRelationshipType: 'SIMILAR', writeProperty: 'score' })",
			map[string]any{}); err != nil {
			return nil, err
		}

		return nil, nil
	})

	if err != nil {
		log.Error("Failed to update user favorites", "err", err)
	}
}

func (s *Service) cleanPosts() {
	// timestamp of 30 days ago
	timestamp := time.Now().AddDate(0, 0, -30).Unix()

	_, err := s.neo4j.ExecuteWrite(func(tx neo4j.ManagedTransaction) (any, error) {
		ctx := context.Background()

		if _, err := tx.Run(ctx, "call apoc.periodic.iterate('match (r:Post) where r.created_at < $Timestamp return r', 'detach delete r', {batchSize:10000, iterateList:true, parallel:false})",
			map[string]any{
				"Timestamp": timestamp,
			}); err != nil {
			return nil, err
		}

		if _, err := tx.Run(ctx, "call apoc.periodic.iterate('match (u:User) where not (u)--() return u', 'detach delete u', {batchSize:10000, iterateList:true, parallel:false})",
			map[string]any{}); err != nil {
			return nil, err
		}

		return nil, nil
	})

	if err != nil {
		log.Error("Failed to batch delete old posts and inactive users", "err", err)
	}
}

func (s *Service) cleanObjs() {
	// delete files older than 7 days
	filepath.Walk(s.config.Objects.Root, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		if time.Since(info.ModTime()) > 7*24*time.Hour {
			_ = os.Remove(path)
		}

		return nil
	})
}

func (s *Service) writeObject(event *nostr.Event) error {
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}

	path, dir := s.objPath(event.ID)
	os.MkdirAll(dir, 0755)
	return os.WriteFile(path, raw, 0644)
}

func (s *Service) readObject(id string) (string, error) {
	file, _ := s.objPath(id)
	bytes, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (s *Service) objPath(id string) (file string, dir string) {
	prefix := id[:3]
	name := id[3:]
	file = path.Join(s.config.Objects.Root, "objects", prefix, name)
	dir = path.Join(s.config.Objects.Root, "objects", prefix)
	return
}

func (s *Service) CreateSubscriber(pubkey, channelSK string, subscribedAt time.Time) error {
	logger.Debug("Create subscriber", "pubkey", pubkey)
	_, err := s.neo4j.ExecuteWrite(func(tx neo4j.ManagedTransaction) (any, error) {
		query := `
			MERGE (s:Subscriber {pubkey: $Pubkey}) ON CREATE
			SET
				s.channel_secret = $ChannelSecret,
				s.subscribed_at = $SubscribedAt,
				s.unsubscribed_at = null;
		`
		_, err := tx.Run(context.Background(), query,
			map[string]any{
				"Pubkey":        pubkey,
				"ChannelSecret": channelSK,
				"SubscribedAt":  subscribedAt.Unix(),
			})
		return nil, err
	})
	return err
}

func (s *Service) ListSubscribers(ctx context.Context, limit, skip int) ([]types.Subscriber, error) {
	subscribers, err := s.neo4j.ExecuteRead(func(tx neo4j.ManagedTransaction) (any, error) {
		ctx := context.Background()

		query := `
		  MATCH (s:Subscriber)
			RETURN s
			ORDER BY s.pubkey
			SKIP $Skip
			LIMIT $Limit;
		`
		result, err := tx.Run(ctx, query,
			map[string]any{
				"Limit": limit,
				"Skip":  skip,
			})
		if err != nil {
			return nil, err
		}

		var subscribers []types.Subscriber

		for result.Next(ctx) {
			record := result.Record()

			rawItemNode, found := record.Get("s")
			if !found {
				return nil, fmt.Errorf("no s field")
			}
			itemNode := rawItemNode.(neo4j.Node)
			props := itemNode.Props

			subscriber := types.Subscriber{
				Pubkey:        props["pubkey"].(string),
				ChannelSecret: props["channel_secret"].(string),
				SubscribedAt: func() *time.Time {
					t := time.Unix(props["subscribed_at"].(int64), 0)
					return &t
				}(),
				UnsubscribedAt: func() *time.Time {
					if v, ok := props["unsubscribed_at"].(int64); ok {
						t := time.Unix(v, 0)
						return &t
					}

					return nil
				}(),
			}

			subscribers = append(subscribers, subscriber)
		}

		return subscribers, nil
	})

	if err != nil {
		return nil, err
	}

	return subscribers.([]types.Subscriber), nil
}

func (s *Service) GetSubscriber(pubkey string) *types.Subscriber {
	subscriber, err := s.neo4j.ExecuteRead(func(tx neo4j.ManagedTransaction) (any, error) {
		ctx := context.Background()

		query := `
			MATCH (s:Subscriber {pubkey: $Pubkey})
			RETURN s;
		`
		result, err := tx.Run(ctx, query,
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

		rawItemNode, found := record.Get("s")
		if !found {
			return nil, fmt.Errorf("no s field")
		}
		itemNode := rawItemNode.(neo4j.Node)
		props := itemNode.Props

		subscriber := types.Subscriber{
			Pubkey:        props["pubkey"].(string),
			ChannelSecret: props["channel_secret"].(string),
			SubscribedAt: func() *time.Time {
				t := time.Unix(props["subscribed_at"].(int64), 0)
				return &t
			}(),
			UnsubscribedAt: func() *time.Time {
				if v, ok := props["unsubscribed_at"].(int64); ok {
					t := time.Unix(v, 0)
					return &t
				}

				return nil
			}(),
		}

		return subscriber, nil
	})

	if err != nil {
		logger.Error("Failed to get subscriber", "err", err)
		return nil
	}

	if result, ok := subscriber.(types.Subscriber); ok {
		return &result
	}

	return nil
}

func (s *Service) DeleteSubscriber(pubkey string, unsubscribedAt time.Time) error {
	logger.Debug("Deleting subscriber", "pubkey", pubkey)
	_, err := s.neo4j.ExecuteWrite(func(tx neo4j.ManagedTransaction) (any, error) {
		query := `
			MATCH (s:Subscriber {pubkey: $Pubkey})
			SET
				s.unsubscribed_at = $UnsubscribedAt;
		`
		_, err := tx.Run(context.Background(), query,
			map[string]any{
				"Pubkey":         pubkey,
				"UnsubscribedAt": unsubscribedAt.Unix(),
			})
		return nil, err
	})
	return err
}

func (s *Service) RestoreSubscriber(pubkey string, subscribedAt time.Time) (bool, error) {
	logger.Debug("Restore subscriber", "pubkey", pubkey)

	subscriber := s.GetSubscriber(pubkey)
	// if unsubscribed_at is null, it means that the subscriber is still subscribed
	if subscriber.UnsubscribedAt == nil {
		return false, nil
	}

	// remove unsubscribed_at timestamp and update subscribed_at timestamp
	_, err := s.neo4j.ExecuteWrite(func(tx neo4j.ManagedTransaction) (any, error) {
		query := `
			MATCH (s:Subscriber {pubkey: $Pubkey})
			SET
				s.unsubscribed_at = null, 
				s.subscribed_at = $SubscribedAt;
		`
		_, err := tx.Run(context.Background(), query,
			map[string]any{
				"Pubkey":       pubkey,
				"SubscribedAt": subscribedAt.Unix(),
			})

		return nil, err
	})

	if err != nil {
		return false, err
	}

	// if the restoring succeeded, return true
	return true, err
}
