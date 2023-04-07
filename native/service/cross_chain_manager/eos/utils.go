package eos

import (
	"bytes"
	"fmt"

	"github.com/polynetwork/poly/common/log"
	"github.com/polynetwork/poly/native"
	cmanager "github.com/polynetwork/poly/native/service/governance/side_chain_manager"
	eossync "github.com/polynetwork/poly/native/service/header_sync/eos"
)

func verifyFromEOSTx(native *native.NativeService, proofByte []byte, fromChainID uint64, height uint32, sideChain *cmanager.SideChain) (bool, error) {
	proof := new(EOSProof)

	log.Errorf("proofByte: %v", proofByte)
	err := proof.Deserialization(proofByte)

	if err != nil {
		fmt.Printf("Deserialization error: %v", err)
	}
	if err != nil {
		return false, fmt.Errorf("VerifyFromEOSProof Deserialization proof error: %v", err)
	}
	log.Errorf("25proofByte: %v", proofByte)
	log.Errorf("proof: %v", proof)

	calRoot := VerifyProof(proof.path, proof.leaf)
	log.Errorf("28proofByte: %v", proofByte)
	headerStore, err := eossync.GetHeaderByHeight(native, uint64(height), fromChainID)
	if err != nil {
		return false, fmt.Errorf("VerifyFromEOSProof Get Header By Height error: %v", err)
	}
	if bytes.Equal(headerStore.TransactionMRoot, calRoot) {
		return true, nil
	}
	return false, fmt.Errorf("VerifyFromEOSProof Get Header Height is %d ,Proof is %v,caculate Proof is %v", height, headerStore.TransactionMRoot, calRoot)
}
