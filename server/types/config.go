package types

type CrawlerConfig struct {
	Relays []string
}

type Neo4jConfig struct {
	Url      string
	Username string
	Password string
}

type LogConfig struct {
	Level string `default:"info"`
	Path  string `default:"console"`
}

type Config struct {
	Log     LogConfig
	Neo4j   Neo4jConfig
	Crawler CrawlerConfig
}
