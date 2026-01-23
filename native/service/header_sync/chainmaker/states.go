package chainmaker

import (
	"fmt"
	pcom "github.com/polynetwork/poly/common"
)

type ChainMakerRoot struct {
	PublicKey []byte
}

func (root *ChainMakerRoot) Serialization(sink *pcom.ZeroCopySink) {
	sink.WriteVarBytes(root.PublicKey)
}

func (root *ChainMakerRoot) Deserialization(source *pcom.ZeroCopySource) error {
	raw, eof := source.NextVarBytes()
	if eof {
		return fmt.Errorf("failed to deserialize RootCA")
	}

	root.PublicKey = raw

	return nil
}
