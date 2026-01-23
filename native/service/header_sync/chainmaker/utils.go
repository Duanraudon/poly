package chainmaker

import (
	"fmt"
	pcom "github.com/polynetwork/poly/common"
	"github.com/polynetwork/poly/core/states"
	"github.com/polynetwork/poly/native"
	"github.com/polynetwork/poly/native/service/header_sync/common"
	"github.com/polynetwork/poly/native/service/utils"
)

func PutChainMakerRoot(native *native.NativeService, root *ChainMakerRoot, chainID uint64) error {
	contract := utils.HeaderSyncContractAddress
	sink := pcom.NewZeroCopySink(nil)
	root.Serialization(sink)
	native.GetCacheDB().Put(utils.ConcatKey(contract, []byte(common.ROOT_CERT), utils.GetUint64Bytes(chainID)),
		states.GenRawStorageItem(sink.Bytes()))

	// Notify about the public keys (simplified version)
	return nil
}

func GetChainMakerRoot(native *native.NativeService, chainID uint64) (*ChainMakerRoot, error) {
	store, err := native.GetCacheDB().Get(utils.ConcatKey(utils.HeaderSyncContractAddress,
		[]byte(common.ROOT_CERT), utils.GetUint64Bytes(chainID)))
	if err != nil {
		return nil, fmt.Errorf("GetChainMakerRoot, get root error: %v", err)
	}
	if store == nil {
		return nil, fmt.Errorf("GetChainMakerRoot, can not find any records")
	}
	raw, err := states.GetValueFromRawStorageItem(store)
	if err != nil {
		return nil, fmt.Errorf("GetChainMakerRoot, deserialize from raw storage item err: %v", err)
	}
	root := &ChainMakerRoot{}
	if err = root.Deserialization(pcom.NewZeroCopySource(raw)); err != nil {
		return nil, fmt.Errorf("GetChainMakerRoot, failed to deserialize ChainMakerRoot: %v", err)
	}
	return root, nil
}
