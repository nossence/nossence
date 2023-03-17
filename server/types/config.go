package types

type BotConfig struct {
	SK     string
	Relays []string
}

type CrawlerConfig struct {
	Relays []string
	Since  string `default:"-1h"`
	Limit  int `default:"0"`
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
	Bot     BotConfig
}
