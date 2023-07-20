package eos

import (
	"fmt"

	"github.com/polynetwork/poly/common"
	"github.com/polynetwork/poly/native"
	scom "github.com/polynetwork/poly/native/service/cross_chain_manager/common"
	"github.com/polynetwork/poly/native/service/governance/side_chain_manager"
)

type EOSHandler struct {
}

func NewEOSHandler() *EOSHandler {
	return &EOSHandler{}
}

func (this *EOSHandler) MakeDepositProposal(service *native.NativeService) (*scom.MakeTxParam, error) {
	params := new(scom.EntranceParam)

	if err := params.Deserialization(common.NewZeroCopySource(service.GetInput())); err != nil {
		return nil, fmt.Errorf("EOS MakeDepositProposal, contract params deserialize error: %s", err)
	}

	sideChain, err := side_chain_manager.GetSideChain(service, params.SourceChainID)
	if err != nil {
		return nil, fmt.Errorf("EOS MakeDepositProposal, side_chain_manager.GetSideChain error: %v", err)
	}

	// 验证默克尔证明，并返回
	isEqual, err := verifyFromEOSTx(service, params.Proof, params.SourceChainID, params.Height, sideChain)
	if err != nil {
		return nil, fmt.Errorf("EOS MakeDepositProposal, verifyFromEOSTx error: %s", err)
	}
	if !isEqual {
		return nil, fmt.Errorf("EOS MakeDepositProposal,proofVerify error,params.Proof%v", params.Proof)
	}
	data := common.NewZeroCopySource(params.Extra)
	txParam := new(scom.MakeTxParam) //
	if err := txParam.Deserialization(data); err != nil {
		return nil, fmt.Errorf("EOS MakeDepositProposal, deserialize txvalue error:%s", err)
	}

	if err := scom.CheckDoneTx(service, txParam.CrossChainID, params.SourceChainID); err != nil {
		return nil, fmt.Errorf("EOS MakeDepositProposal, check done transaction error: %s", err)
	}
	if err := scom.PutDoneTx(service, txParam.CrossChainID, params.SourceChainID); err != nil {
		return nil, fmt.Errorf("EOS MakeDepositProposal, PutDoneTx error: %s", err)
	}

	return txParam, nil
}
