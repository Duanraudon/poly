package eos

import (
	"fmt"
	"time"

	"github.com/polynetwork/poly/native"
	scom "github.com/polynetwork/poly/native/service/header_sync/common"
	"github.com/polynetwork/poly/native/service/utils"

	cstates "github.com/polynetwork/poly/core/states"
	eos "github.com/qqtou/eos-go"
)

const (
	allowedFutureBlockTime = 500 * time.Millisecond // 0.5s
)

func putGenesisBlockHeader(native *native.NativeService, blockHeader *eos.SignedBlockHeader, chainID uint64) error {

	contract := utils.HeaderSyncContractAddress
	blockID, _ := blockHeader.BlockID()
	blockIDBytes, _ := blockID.MarshalJSON()

	storeBytes, _ := eos.MarshalBinary(blockHeader)
	// GENESIS_HEADER => the genesis header byte code
	native.GetCacheDB().Put(utils.ConcatKey(contract, []byte(scom.GENESIS_HEADER), utils.GetUint64Bytes(chainID)),
		cstates.GenRawStorageItem(storeBytes))
	// HEADER_INDEX => the mapping of block id and block header byte code, for querying block header by its id
	native.GetCacheDB().Put(utils.ConcatKey(contract, []byte(scom.HEADER_INDEX), utils.GetUint64Bytes(chainID), blockIDBytes),
		cstates.GenRawStorageItem(storeBytes))
	// MAIN_CHAIN => the mapping of block height and block header id, for querying block header id by its height
	native.GetCacheDB().Put(utils.ConcatKey(contract, []byte(scom.MAIN_CHAIN), utils.GetUint64Bytes(chainID), utils.GetUint64Bytes(uint64(blockHeader.BlockNumber()))),
		cstates.GenRawStorageItem(blockIDBytes))
	// CURRENT_HEADER_HEIGHT => current block hieght of side chain in poly relay chain
	native.GetCacheDB().Put(utils.ConcatKey(contract, []byte(scom.CURRENT_HEADER_HEIGHT), utils.GetUint64Bytes(chainID)),
		cstates.GenRawStorageItem(utils.GetUint64Bytes(uint64(blockHeader.BlockNumber()))))
	// notify
	scom.NotifyPutHeader(native, chainID, uint64(blockHeader.BlockNumber()), blockID.String())
	return nil
}

func putBlockHeader(native *native.NativeService, blockHeader *eos.SignedBlockHeader, chainID uint64) error {
	contract := utils.HeaderSyncContractAddress
	blockID, err := blockHeader.BlockID()
	if err != nil {
		return fmt.Errorf("putBlockHeader, get header BlockID error:%v", err)
	}
	blockIDBytes, err := blockID.MarshalJSON()
	if err != nil {
		return fmt.Errorf("putBlockHeader, MarshalJSON BlockID error:%v", err)
	}
	storeBytes, err := eos.MarshalBinary(blockHeader)
	if err != nil {
		return fmt.Errorf("putBlockHeader, MarshalBinary blockHeader error:%v", err)
	}
	// HEADER_INDEX => the mapping of block ID and block header byte code, for querying block header by its ID
	native.GetCacheDB().Put(utils.ConcatKey(contract, []byte(scom.HEADER_INDEX), utils.GetUint64Bytes(chainID), blockIDBytes),
		cstates.GenRawStorageItem(storeBytes))
	scom.NotifyPutHeader(native, chainID, uint64(blockHeader.BlockNumber()), blockID.String())
	return nil
}

// appendHeader2Main stores the mapping between block height and block header id with key MAIN_CHAIN and update the CURRENT_HEADER_HEIGHT
func appendHeader2Main(native *native.NativeService, height uint64, blockIDBytes []byte, chainID uint64) error {
	contract := utils.HeaderSyncContractAddress
	native.GetCacheDB().Put(utils.ConcatKey(contract, []byte(scom.MAIN_CHAIN), utils.GetUint64Bytes(chainID), utils.GetUint64Bytes(height)),
		cstates.GenRawStorageItem(blockIDBytes))
	native.GetCacheDB().Put(utils.ConcatKey(contract, []byte(scom.CURRENT_HEADER_HEIGHT), utils.GetUint64Bytes(chainID)),
		cstates.GenRawStorageItem(utils.GetUint64Bytes(height)))
	scom.NotifyPutHeader(native, chainID, height, string(blockIDBytes))
	return nil
}

func GetCurrentHeader(native *native.NativeService, chainID uint64) (*eos.SignedBlockHeader, error) {
	height, err := GetCurrentHeaderHeight(native, chainID)
	if err != nil {
		return nil, err
	}
	header, err := GetHeaderByHeight(native, height, chainID)
	if err != nil {
		return nil, err
	}
	return header, nil
}

func GetHeaderByHeight(native *native.NativeService, height, chainID uint64) (*eos.SignedBlockHeader, error) {
	lastestHeight, err := GetCurrentHeaderHeight(native, chainID)
	if err != nil {
		return nil, err
	}
	if height > lastestHeight {
		return nil, fmt.Errorf("GetHeaderByHeight, height is too big")
	}
	IDStore, err := native.GetCacheDB().Get(utils.ConcatKey(utils.HeaderSyncContractAddress,
		[]byte(scom.MAIN_CHAIN), utils.GetUint64Bytes(chainID), utils.GetUint64Bytes(height)))
	if err != nil {
		return nil, fmt.Errorf("GetHeaderByHeight, get blockIDStore error: %v", err)
	}
	if IDStore == nil {
		return nil, fmt.Errorf("GetHeaderByHeight, can not find any header records")
	}
	IDBytes, err := cstates.GetValueFromRawStorageItem(IDStore)
	if err != nil {
		return nil, fmt.Errorf("GetHeaderByHeight, deserialize IDBytes from raw storage item err: %v", err)
	}
	return GetHeaderByID(native, IDBytes, chainID)
}

func GetCurrentHeaderHeight(native *native.NativeService, chainID uint64) (uint64, error) {
	heightStore, err := native.GetCacheDB().Get(utils.ConcatKey(utils.HeaderSyncContractAddress,
		[]byte(scom.CURRENT_HEADER_HEIGHT), utils.GetUint64Bytes(chainID)))
	if err != nil {
		return 0, fmt.Errorf("getPreHeaderHeight error: %v", err)
	}
	if heightStore == nil {
		return 0, fmt.Errorf("getPreHeaderHeight, heightStore is nil")
	}
	hieghtBytes, err := cstates.GetValueFromRawStorageItem(heightStore)
	if err != nil {
		return 0, fmt.Errorf("getHeaderByHeight, deserialize headerBytes from raw storage item err: %v", err)
	}
	return utils.GetBytesUint64(hieghtBytes), err
}

func GetHeaderByID(native *native.NativeService, blockIDBytes []byte, chainID uint64) (*eos.SignedBlockHeader, error) {
	headerStore, err := native.GetCacheDB().Get(utils.ConcatKey(utils.HeaderSyncContractAddress,
		[]byte(scom.HEADER_INDEX), utils.GetUint64Bytes(chainID), blockIDBytes))
	if err != nil {
		return nil, fmt.Errorf("GetHeaderByID, get blockHeaderStore error: %v", err)
	}
	if headerStore == nil {
		return nil, fmt.Errorf("GetHeaderByID, can not find any header records")
	}
	storeBytes, err := cstates.GetValueFromRawStorageItem(headerStore)
	if err != nil {
		return nil, fmt.Errorf("GetHeaderById, diserialize headerBytes from raw storage item err: %v", err)
	}
	var header *eos.SignedBlockHeader
	if err := eos.UnmarshalBinary(storeBytes, &header); err != nil {
		return nil, fmt.Errorf("GetHeaderById, diserialize header error: %v", err)
	}
	return header, nil
}

func IsHeaderExist(native *native.NativeService, blockIDByte []byte, chainID uint64) (bool, error) {
	headerStore, err := native.GetCacheDB().Get(utils.ConcatKey(utils.HeaderSyncContractAddress,
		[]byte(scom.HEADER_INDEX), utils.GetUint64Bytes(chainID), blockIDByte))
	if err != nil {
		return false, fmt.Errorf("IsHeaderExist, get blockHashStore error: %v", err)
	}
	if headerStore == nil {
		return false, nil
	} else {
		return true, nil
	}
}
