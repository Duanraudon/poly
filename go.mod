module github.com/polynetwork/poly

go 1.16

require (
	github.com/btcsuite/btcd v0.20.1-beta
	github.com/btcsuite/btcutil v1.0.2
	github.com/cosmos/cosmos-sdk v0.39.0
	github.com/ethereum/go-ethereum v1.10.15
	github.com/gcash/bchd v0.16.5
	github.com/gcash/bchutil v0.0.0-20200506001747-c2894cd54b33
	github.com/gorilla/websocket v1.4.2
	github.com/gosuri/uiprogress v0.0.1
	github.com/hashicorp/golang-lru v0.5.5-0.20210104140557-80c98217689d
	github.com/howeyc/gopass v0.0.0-20190910152052-7cb4b85ec19c
	github.com/itchyny/base58-go v0.1.0
	github.com/joeqian10/neo-gogogo v0.0.0-20200716075409-923bd4879b43
	github.com/ontio/ontology v1.11.0
	github.com/ontio/ontology-crypto v1.0.9
	github.com/ontio/ontology-eventbus v0.9.1
	github.com/pborman/uuid v1.2.0
	github.com/stretchr/testify v1.7.0
	github.com/syndtr/goleveldb v1.0.1-0.20210819022825-2ae1ddf74ef7
	github.com/tendermint/tendermint v0.33.6
	github.com/tjfoc/gmsm v1.3.2-0.20200914155643-24d14c7bd05c
	github.com/urfave/cli v1.22.4
	github.com/valyala/bytebufferpool v1.0.0
	golang.org/x/crypto v0.0.0-20210322153248-0c34fe9e7dc2
	golang.org/x/net v0.0.0-20210805182204-aaa1db679c0d
	gotest.tools v2.2.0+incompatible
)

replace github.com/tjfoc/gmsm => github.com/zouxyan/gmsm v1.3.2-0.20200925082225-a66aabdb8da8
