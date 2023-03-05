package database

import (
	"context"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type Neo4jDb struct {
	driver neo4j.DriverWithContext
}

func NewNeo4jDb() *Neo4jDb {
	return &Neo4jDb{}
}

func (db *Neo4jDb) Connect() error {
	// TODO: read username & password from config
	driver, err := neo4j.NewDriverWithContext("bolt://localhost:7687", neo4j.BasicAuth("neo4j", "12345678", ""))
	if err != nil {
		return err
	}

	db.driver = driver
	return nil
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
