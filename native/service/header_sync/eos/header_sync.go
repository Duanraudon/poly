package eos

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/polynetwork/poly/common/log"

	"github.com/polynetwork/poly/common"
	"github.com/polynetwork/poly/native"
	"github.com/polynetwork/poly/native/service/governance/node_manager"
	scom "github.com/polynetwork/poly/native/service/header_sync/common"
	"github.com/polynetwork/poly/native/service/utils"
	eos "github.com/qqtou/eos-go"
)

type EOSHandler struct {
}

func NewEOSHandler() *EOSHandler {
	return &EOSHandler{}
}

func (this *EOSHandler) SyncGenesisHeader(native *native.NativeService) error {
	params := new(scom.SyncGenesisHeaderParam)
	if err := params.Deserialization(common.NewZeroCopySource(native.GetInput())); err != nil {
		return fmt.Errorf("EOSHandler SyncGenesisHeader, contract params deserialize error: %v", err)
	}
	// Get current epoch operator
	operatorAddress, err := node_manager.GetCurConOperator(native)
	if err != nil {
		return fmt.Errorf("EOSHeander SyncGenesisHeader, get current consensus operator address error: %v", err)
	}
	// check witness, ensure the legitimate of the poly native service data
	err = utils.ValidateOwner(native, operatorAddress)
	if err != nil {
		return fmt.Errorf("EOSHandler SyncGenesisHeader: %s", err)
	}
	// parse header data from native service input field
	header, err := getGenesisHeader(native.GetInput())
	if err != nil {
		return fmt.Errorf("EOSHeader SyncGenesisHeader: %s", err)
	}
	// look for genesis header from relay chain to make sure func SyncGenesisHeader is only called once
	headerStore, err := native.GetCacheDB().Get(utils.ConcatKey(utils.HeaderSyncContractAddress, []byte(scom.GENESIS_HEADER), utils.GetUint64Bytes(params.ChainID)))
	if err != nil {
		return fmt.Errorf("EOSHeader GetHeaderByHeight, get blockHashStore error: %v", err)
	}
	if headerStore != nil {
		return fmt.Errorf("EOSHandler GetHeaderByHeight, genesis header had been initialized")
	}
	// store the information of genesis header to poly chain
	err = putGenesisBlockHeader(native, header, params.ChainID)
	if err != nil {
		return fmt.Errorf("EOSHandler SyncGenesisHeader, put blockHeader error: %v", err)
	}
	return nil
}

func (this *EOSHandler) SyncBlockHeader(native *native.NativeService) error {
	headerParams := new(scom.SyncBlockHeaderParam)
	if err := headerParams.Deserialization(common.NewZeroCopySource(native.GetInput())); err != nil {
		return fmt.Errorf("SyncBlockHeader, contract params deserialize error: %v", err)
	}

	var header *eos.SignedBlockHeader

	for _, v := range headerParams.Headers {
		err := eos.UnmarshalBinary(v, &header)
		if err != nil {
			return fmt.Errorf("SyncBlockHeader, deserialize header err: %v", err)
		}
		blockID, err := header.BlockID() //hdrBlockID
		if err != nil {
			return fmt.Errorf("SyncBlockHeader, Get Header BlockID error:%v", err)
		}
		blockIDBytes, _ := blockID.MarshalJSON() //hdrBlockIDBytes
		if err != nil {
			return fmt.Errorf("SyncBlockHeader, MarshalJSON Header BlockID error:%v", err)
		}
		preID := header.Previous               // hdrPreID
		preIDBytes, err := preID.MarshalJSON() //hdrPreIdBytes
		if err != nil {
			return fmt.Errorf("SyncBlockHeader, MarshalJSON Header Previous error:%v", err)
		}
		exist, err := IsHeaderExist(native, blockIDBytes, headerParams.ChainID)
		if err != nil {
			return fmt.Errorf("SyncBlockHeader, check header exist err: %v", err)
		}
		if exist == true {
			log.Warnf("SyncBlockHeader, header has exist. Header: %s", hex.EncodeToString(v))
			continue
		}
		// 获取pre header
		parentHeader, err := GetHeaderByID(native, preIDBytes, headerParams.ChainID)
		if err != nil {
			return fmt.Errorf("SyncBlockHeader, get the parent block faild. Error:%s, header: %s,headerPreID is: %v,headerID is:%v,header height is: %v", err, hex.EncodeToString(v), preID, blockID, header.BlockNumber())
		}
		log.Warnf("SyncBlockHeader, parentHeader is:%v", parentHeader)
		parentHeaderId, err := parentHeader.BlockID()          // preHdrID
		parentHeaderIdBytes, _ := parentHeaderId.MarshalJSON() // preHdrIDBytes

		/*
			验证区块安全性
		*/
		// verify whether current height is parent height plus one
		if uint64(header.BlockNumber()) != uint64(parentHeader.BlockNumber())+1 {
			return fmt.Errorf("SyncBlockHeader, invalid header height: %d parent height: %d", uint64(header.BlockNumber()), uint64(parentHeader.BlockNumber()))
		}
		// verify whether parent hash validity
		if !bytes.Equal(parentHeaderIdBytes, preIDBytes) {
			return fmt.Errorf("SyncBlockHeader, headerId is not equal,\n Header: %v\n,Parent header: %v\n, header Previous is:%v\n, Parent header ID is:%v\n header Previous Bytes is :%v\n, Parent headr ID Bytes is%v\n, ID judge Equal:%v\n,ID Bytes judge Equal:%v\n", header, parentHeader, preID, parentHeaderId, preIDBytes, parentHeaderIdBytes, bytes.Equal(preID, parentHeaderId), bytes.Equal(preIDBytes, parentHeaderIdBytes))
		}
		// verify current time validity
		if header.Timestamp.Time.UnixNano() != parentHeader.Timestamp.Add(allowedFutureBlockTime).UnixNano() {
			log.Warn("SyncBlockHeader time is not continuous, current time is unvalidity, header Time: %v\n,Parent header Time: %v\n,time is Equal: %v,block number is %d", header.Timestamp.Time.UnixNano(), parentHeader.Timestamp.Time.UnixNano(), header.Timestamp.Time.UnixNano() == parentHeader.Timestamp.Add(allowedFutureBlockTime).UnixNano(), header.BlockNumber())
		}
		// err = this.verifyHeader(header)
		// if err != nil {
		// 	return fmt.Errorf("SyncBlockHeader, verify header error: %v, header: %s", err, string(v))
		// }

		// block header storage store the mapping between block header ID and header data into poly
		err = putBlockHeader(native, header, headerParams.ChainID)
		if err != nil {
			return fmt.Errorf("SyncGenesisHeader, put blockHeader error: %v, header: %v", err, header)
		}
		// get current header of main
		currentHeader, err := GetCurrentHeader(native, headerParams.ChainID)
		if err != nil {
			return fmt.Errorf("SyncBlockHeader, get the current block failed. error:%s", err)
		}
		//make sure the block header synchronization is ordered
		currentHeaderID, _ := currentHeader.BlockID()
		currentHeaderIDBytes, _ := currentHeaderID.MarshalJSON()
		if bytes.Equal(currentHeaderIDBytes, preIDBytes) {
			//if header is just the next one
			appendHeader2Main(native, uint64(header.BlockNumber()), blockIDBytes, headerParams.ChainID)
		}
	}
	return nil
}

func getGenesisHeader(input []byte) (*eos.SignedBlockHeader, error) {
	params := new(scom.SyncGenesisHeaderParam)
	if err := params.Deserialization(common.NewZeroCopySource(input)); err != nil {
		return nil, fmt.Errorf("getGenesisHeader, contract params deserialize error: %v", err)
	}
	var header *eos.SignedBlockHeader
	err := eos.UnmarshalBinary(params.GenesisHeader, &header)
	if err != nil {
		return nil, fmt.Errorf("getGenesisHeader, deserialize header err: %v", err)
	}
	return header, nil
}

// func (this *EOSHandler) verifyHeader(header *eos.SignedBlockHeader) error {
// 	return nil
// }

func (this *EOSHandler) SyncCrossChainMsg(native *native.NativeService) error {
	return nil
}
