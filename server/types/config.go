package types

type BotConfig struct {
	SK       string
	Relays   []string
	Metadata MetadataConfig
}

type MetadataConfig struct {
	Name           string `default:"nossence"`
	About          string
	Picture        string
	Nip05          string
	ChannelName    string `default:"nossence curator"`
	ChannelAbout   string
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

type ObjectsConfig struct {
	Root string `default:"/var/data/nossence"`
}

type Config struct {
	Log     LogConfig
	Neo4j   Neo4jConfig
	Crawler CrawlerConfig
	Objects ObjectsConfig
	Bot     BotConfig
}
