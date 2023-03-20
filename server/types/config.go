package types

type BotConfig struct {
	SK       string
	Relays   []string
	Metadata MetadataConfig
}

type MetadataConfig struct {
	Name           string `default:"nossence"`
	About          string `default:"a recommender engine for nostr"`
	Picture        string
	Nip05          string
	ChannelName    string `default:"nossence curator"`
	ChannelAbout   string `default:"nossence curated content for %s"`
	ChannelPicture string
}

type CrawlerConfig struct {
	Relays []string
	Since  string `default:"-1h"`
	Limit  int    `default:"0"`
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
