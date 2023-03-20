package nostr

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/dyng/nosdaily/database"
	"github.com/dyng/nosdaily/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/nbd-wtf/go-nostr"
)

type NameServer struct {
	config   *types.Config
	neo4j    *database.Neo4jDb
	mainPub  string
	mainName string
}

type NameResponse struct {
	Names map[string]string `json:"names"`
}

func NewNameServer(config *types.Config, neo4j *database.Neo4jDb) *NameServer {
	pub, err := nostr.GetPublicKey(config.Bot.SK)
	if err != nil {
		log.Crit("failed to get public key", "err", err)
	}

	mainName := strings.Split(config.Bot.Metadata.Name, "@")[0]

	return &NameServer{
		config:  config,
		neo4j:   neo4j,
		mainPub: pub,
		mainName: mainName,
	}
}

func (ns *NameServer) Serve(w http.ResponseWriter, r *http.Request) {
	names := r.URL.Query()["name"]

	resp := NameResponse{Names: make(map[string]string)}
	for _, name := range names {
		if name == ns.mainName {
			resp.Names[name] = ns.mainPub
		} else {
			// TODO
			log.Warn("name not found", "name", name)
		}
	}

	json.NewEncoder(w).Encode(resp)
}
