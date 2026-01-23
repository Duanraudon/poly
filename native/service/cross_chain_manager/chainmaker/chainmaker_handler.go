/*
 * Copyright (C) 2020 The poly network Authors
 * This file is part of The poly network library.
 *
 * The  poly network  is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * The  poly network  is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 * You should have received a copy of the GNU Lesser General Public License
 * along with The poly network .  If not, see <http://www.gnu.org/licenses/>.
 */
package chainmaker

import (
	"fmt"
	pcom "github.com/polynetwork/poly/common"
	"github.com/polynetwork/poly/native"
	"github.com/polynetwork/poly/native/service/cross_chain_manager/common"
	"github.com/polynetwork/poly/native/service/header_sync/chainmaker"
)

type ChainMakerHandler struct{}

func NewChainMakerHandler() *ChainMakerHandler {
	return &ChainMakerHandler{}
}

// MakeDepositProposal
// @Description: 每次接受的跨链消息，验证签名（加签验签用的是chainMaker-relayer配置的节点私钥，默认使用node1的）
// @receiver this
// @param ns
// @return *common.MakeTxParam
// @return error
func (c *ChainMakerHandler) MakeDepositProposal(ns *native.NativeService) (*common.MakeTxParam, error) {
	params := new(common.EntranceParam)
	if err := params.Deserialization(pcom.NewZeroCopySource(ns.GetInput())); err != nil {
		return nil, fmt.Errorf("ChainMaker MakeDepositProposal, contract params deserialize error: %v", err)
	}
	val := &common.MakeTxParam{}
	if err := val.Deserialization(pcom.NewZeroCopySource(params.Extra)); err != nil {
		return nil, fmt.Errorf("ChainMaker MakeDepositProposal, failed to deserialize MakeTxParam: %v", err)
	}

	if err := common.CheckDoneTx(ns, val.CrossChainID, params.SourceChainID); err != nil {
		return nil, fmt.Errorf("ChainMaker MakeDepositProposal, check done transaction error: %v", err)
	}

	// Get node public keys
	root, err := chainmaker.GetChainMakerRoot(ns, params.SourceChainID)
	if err != nil {
		return nil, fmt.Errorf("ChainMaker MakeDepositProposal, failed to get the chainmaker node public keys: %v", err)
	}

	result, err := chainmaker.VerifySignature(params.Extra, params.Proof, root.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("ChainMaker MakeDepositProposal, failed to check sig: %v", err)
	}

	if !result {
		return nil, fmt.Errorf("ChainMaker MakeDepositProposal, failed to check sig")
	}

	// Update latest processing height
	PutChainMakerLatestHeightInProcessing(ns, params.SourceChainID, val.FromContractAddress, params.Height)

	if err = common.PutDoneTx(ns, val.CrossChainID, params.SourceChainID); err != nil {
		return nil, fmt.Errorf("ChainMaker MakeDepositProposal, PutDoneTx error: %v", err)
	}

	return val, nil
}
