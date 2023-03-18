package database

import (
	"context"

	"github.com/dyng/nosdaily/types"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type Neo4jDb struct {
	config *types.Config
	driver neo4j.DriverWithContext
}

func NewNeo4jDb(config *types.Config) *Neo4jDb {
	return &Neo4jDb{
		config: config,
	}
}

func (db *Neo4jDb) Connect() error {
	conf := db.config.Neo4j

	driver, err := neo4j.NewDriverWithContext(conf.Url, neo4j.BasicAuth(conf.Username, conf.Password, ""))
	if err != nil {
		return err
	}

	db.driver = driver
	return nil
}

func (db *Neo4jDb) GetDriver() neo4j.DriverWithContext {
	return db.driver
}

func (db *Neo4jDb) Close() error {
	return db.driver.Close(context.Background())
}

func (db *Neo4jDb) ExecuteRead(work func(tx neo4j.ManagedTransaction) (any, error)) (any, error) {
	ctx := context.Background()

	session := db.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	return session.ExecuteRead(ctx, work)
}

func (db *Neo4jDb) ExecuteWrite(work func(tx neo4j.ManagedTransaction) (any, error)) (any, error) {
	ctx := context.Background()

	session := db.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	return session.ExecuteWrite(ctx, work)
}

func (db *Neo4jDb) Run(cypher string, params map[string]any) (neo4j.ResultWithContext, error) {
	ctx := context.Background()

	session := db.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})

	return session.Run(ctx, cypher, params)
}
