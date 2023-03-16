package service

import (
	"context"
	"testing"

	"github.com/dyng/nosdaily/database"
	"github.com/dyng/nosdaily/types"
	"github.com/nbd-wtf/go-nostr"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/stretchr/testify/assert"
)

var neo4jdb *database.Neo4jDb
var service *Service

func TestStoreZap(t *testing.T) {
	setup()
	defer teardown()

	// prepare
	postJson := `
{
    "pubkey": "32e1827635450ebb3c5a7d12c1f8e7b2b514439ac10a67eef3d9fd9c5c68e245",
    "content": "The entirety of my mentions every time I open the app is people complaining about damus. Finding myself not wanting to open the app as much these days. I have been working 24/7 since December and even pushed an update in the air on the way to CR. Reminder I\u2019m not making any money on this app and its still a passion project is slowly bankrupting me. Sorry if I\u2019m not working hard enough \ud83d\ude15",
    "id": "37e092174c1b387203aa0c62fd302f8425aa0be4816c7ad2890c42a770c05f3f",
    "created_at": 1678454022,
    "sig": "c8036d5674c1644791e32fdf951290f74330821e143727b60eda31f957428780a3923c4d7cf0cf886f447c9528bf2c44335f72e05510a49fb91810bcec62e7be",
    "kind": 1,
    "tags": [
        [
            "e",
            "bd43763dd1213ee3801c5d64030f95b6b4f61d3df1e1c894995005ebaaaf65ec"
        ],
        [
            "e",
            "c8429b369c53432377dd2170b8bfc939a465b4ee1606fa086320dd586b48b334"
        ],
        [
            "p",
            "ecfa3c5c82d589c867c044056f75d6cff794f1886d5ebcdd48ad851da47adae4"
        ],
        [
            "p",
            "b17c59874dc05d7f6ec975bce04770c8b7fa9d37f3ad0096fdb76c9385d68928"
        ]
    ]
}
	`
	post := new(nostr.Event)
	err := post.UnmarshalJSON([]byte(postJson))
	if err != nil {
		t.Fatal(err)
	}
	err = service.StorePost(post)

	zapJson := `
{
	"id": "67b48a14fb66c60c8f9070bdeb37afdfcc3d08ad01989460448e4081eddda446",
	"pubkey": "9630f464cca6a5147aa8a35f0bcdd3ce485324e732fd39e09233b1d848238f31",
	"created_at": 1674164545,
	"kind": 9735,
	"tags": [
	  [
		"p",
		"32e1827635450ebb3c5a7d12c1f8e7b2b514439ac10a67eef3d9fd9c5c68e245"
	  ],
	  [
		"e",
		"37e092174c1b387203aa0c62fd302f8425aa0be4816c7ad2890c42a770c05f3f"
	  ],
	  [
		"bolt11",
		"lnbc10u1p3unwfusp5t9r3yymhpfqculx78u027lxspgxcr2n2987mx2j55nnfs95nxnzqpp5jmrh92pfld78spqs78v9euf2385t83uvpwk9ldrlvf6ch7tpascqhp5zvkrmemgth3tufcvflmzjzfvjt023nazlhljz2n9hattj4f8jq8qxqyjw5qcqpjrzjqtc4fc44feggv7065fqe5m4ytjarg3repr5j9el35xhmtfexc42yczarjuqqfzqqqqqqqqlgqqqqqqgq9q9qxpqysgq079nkq507a5tw7xgttmj4u990j7wfggtrasah5gd4ywfr2pjcn29383tphp4t48gquelz9z78p4cq7ml3nrrphw5w6eckhjwmhezhnqpy6gyf0"
	  ],
	  [
		"description",
		"{\"pubkey\":\"32e1827635450ebb3c5a7d12c1f8e7b2b514439ac10a67eef3d9fd9c5c68e245\",\"content\":\"\",\"id\":\"d9cc14d50fcb8c27539aacf776882942c1a11ea4472f8cdec1dea82fab66279d\",\"created_at\":1674164539,\"sig\":\"77127f636577e9029276be060332ea565deaf89ff215a494ccff16ae3f757065e2bc59b2e8c113dd407917a010b3abd36c8d7ad84c0e3ab7dab3a0b0caa9835d\",\"kind\":9734,\"tags\":[[\"e\",\"3624762a1274dd9636e0c552b53086d70bc88c165bc4dc0f9e836a1eaf86c3b8\"],[\"p\",\"32e1827635450ebb3c5a7d12c1f8e7b2b514439ac10a67eef3d9fd9c5c68e245\"],[\"relays\",\"wss://relay.damus.io\",\"wss://nostr-relay.wlvs.space\",\"wss://nostr.fmt.wiz.biz\",\"wss://relay.nostr.bg\",\"wss://nostr.oxtr.dev\",\"wss://nostr.v0l.io\",\"wss://brb.io\",\"wss://nostr.bitcoiner.social\",\"ws://monad.jb55.com:8080\",\"wss://relay.snort.social\"]]}"
	  ],
	  [
		"preimage",
		"5d006d2cf1e73c7148e7519a4c68adc81642ce0e25a432b2434c99f97344c15f"
	  ]
	],
	"content": "",
	"sig": "b0a3c5c984ceb777ac455b2f659505df51585d5fd97a0ec1fdb5f3347d392080d4b420240434a3afd909207195dac1e2f7e3df26ba862a45afd8bfe101c2b1cc"
}
	`
	zap := new(nostr.Event)
	err = zap.UnmarshalJSON([]byte(zapJson))
	if err != nil {
		t.Fatal(err)
	}

	// process
	err = service.StoreZap(zap)

	// verify
	assert.NoError(t, err)
	amount, err := neo4jdb.ExecuteRead(func(tx neo4j.ManagedTransaction) (any, error) {
		ctx := context.Background()

		result, err := tx.Run(ctx, "MATCH (:Post)-[z:ZAP]->(:Post) RETURN z.amount", nil)
		if err != nil {
			return nil, err
		}
		if result.Next(ctx) {
			record := result.Record()
			return record.Values[0].(int64), nil
		} else {
			return nil, nil
		}
	})
	assert.NoError(t, err)
	assert.Equal(t, int64(1000), amount)
}

func setup() {
	if neo4jdb == nil {
		// TODO: use testcontainer
		config := &types.Config{
			Neo4j: types.Neo4jConfig{
				Url:      "bolt://localhost:7687",
				Username: "neo4j",
				Password: "12345678",
			},
		}
		neo4jdb = database.NewNeo4jDb(config)

		error := neo4jdb.Connect()
		if error != nil {
			panic(error)
		}

		service = NewService(config, neo4jdb)
	}
}

func teardown() {
}
