package eos

import (
	"encoding/json"
	"fmt"

	"github.com/polynetwork/poly/common"
	"github.com/polynetwork/poly/common/log"
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

	jsonStr, _ := json.Marshal(params)
	log.Errorf("params jsonStr:%s\n", jsonStr)
	sideChain, err := side_chain_manager.GetSideChain(service, params.SourceChainID)
	if err != nil {
		return nil, fmt.Errorf("EOS MakeDepositProposal, side_chain_manager.GetSideChain error: %v", err)
	}
	jsonStr, _ = json.Marshal(params)
	log.Errorf("35params jsonStr:%s\n", jsonStr)
	log.Errorf("proofByte: %v", params.Proof)
	// var proofExtra []byte
	// copy(proofExtra, params.Proof)
	// log.Errorf("39proofByte: %v",proofExtra)
	// input2 := make([]byte, len(service.GetInput()))
	// copy(input2, service.GetInput())

	// params2 := new(scom.EntranceParam)

	// if err := params2.Deserialization(common.NewZeroCopySource(input2)); err != nil {
	// 	return nil, fmt.Errorf("EOS MakeDepositProposal, contract params2 deserialize error: %s", err)
	// }

	// proofExtra := make([]byte, len(params.Proof))
	// copy(proofExtra, params.Proof)

	// 验证默克尔证明，并返回
	//isEqual, err := verifyFromEOSTx(service, params2.Proof, params2.SourceChainID, params2.Height, sideChain)
	
	// 验证默克尔证明，并返回
	isEqual, err := verifyFromEOSTx(service, params.Proof, params.SourceChainID, params.Height, sideChain)
	if err != nil {
		return nil, fmt.Errorf("EOS MakeDepositProposal, verifyFromEOSTx error: %s", err)
	}
	if !isEqual {
		return nil, fmt.Errorf("EOS MakeDepositProposal,proofVerify error,params.Proof%v", params.Proof)
	}
	jsonStr, _ = json.Marshal(params)
	log.Errorf("48params jsonStr:%s\n", jsonStr)
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
