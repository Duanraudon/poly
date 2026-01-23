package chainmaker

import (
	"fmt"
	"github.com/polynetwork/poly/common"
	"github.com/polynetwork/poly/native"
	"github.com/polynetwork/poly/native/service/governance/node_manager"
	scom "github.com/polynetwork/poly/native/service/header_sync/common"
	"github.com/polynetwork/poly/native/service/utils"
)

type ChainMakerHandler struct{}

func NewChainMakerHandler() *ChainMakerHandler {
	return &ChainMakerHandler{}
}

// SyncGenesisHeader
// @Description: 从chainmaker-relayer传来公钥并进行存储
// @receiver c
// @param ns
// @return error
func (c *ChainMakerHandler) SyncGenesisHeader(ns *native.NativeService) error {
	params := new(scom.SyncGenesisHeaderParam)
	if err := params.Deserialization(common.NewZeroCopySource(ns.GetInput())); err != nil {
		return fmt.Errorf("ChainMakerHandler SyncGenesisHeader, contract params deserialize error: %v", err)
	}

	// Get current epoch operator
	operatorAddress, err := node_manager.GetCurConOperator(ns)
	if err != nil {
		return fmt.Errorf("ChainMakerHandler SyncGenesisHeader, get current consensus operator address error: %v", err)
	}
	// check witness
	err = utils.ValidateOwner(ns, operatorAddress)
	if err != nil {
		return fmt.Errorf("ChainMakerHandler SyncGenesisHeader, checkWitness error: %v", err)
	}

	root := &ChainMakerRoot{
		PublicKey: params.GenesisHeader,
	}

	// Store the node public keys
	if err = PutChainMakerRoot(ns, root, params.ChainID); err != nil {
		return fmt.Errorf("ChainMakerHandler SyncGenesisHeader, failed to put new chainmaker root into storage: %v", err)
	}

	return nil
}

func (c *ChainMakerHandler) SyncBlockHeader(ns *native.NativeService) error {
	return nil
}

func (c *ChainMakerHandler) SyncCrossChainMsg(ns *native.NativeService) error {
	return nil
}
