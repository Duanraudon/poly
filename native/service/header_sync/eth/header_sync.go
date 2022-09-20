/*
 * Copyright (C) 2021 The poly network Authors
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
package eth

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash"
	"math/big"

	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/polynetwork/poly/common/log"
	"github.com/polynetwork/poly/native/service/governance/node_manager"
	"github.com/polynetwork/poly/native/service/governance/side_chain_manager"
	"golang.org/x/crypto/sha3"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/polynetwork/poly/common"
	"github.com/polynetwork/poly/native"
	scom "github.com/polynetwork/poly/native/service/header_sync/common"
	"github.com/polynetwork/poly/native/service/utils"
)

var (
	BIG_1             = big.NewInt(1)
	BIG_2             = big.NewInt(2)
	BIG_9             = big.NewInt(9)
	BIG_MINUS_99      = big.NewInt(-99)
	BLOCK_DIFF_FACTOR = big.NewInt(2048)
	DIFF_PERIOD       = big.NewInt(100000)
	BOMB_DELAY        = big.NewInt(8999999)
)

// Proof-of-stake protocol constants.
var (
	beaconDifficulty = ethcommon.Big0       // The default block difficulty in the beacon consensus
	beaconNonce      = types.EncodeNonce(0) // The default block nonce in the beacon consensus
)

type ETHHandler struct {
}

func NewETHHandler() *ETHHandler {
	return &ETHHandler{}
}

func (this *ETHHandler) SyncGenesisHeader(native *native.NativeService) error {
	params := new(scom.SyncGenesisHeaderParam)
	if err := params.Deserialization(common.NewZeroCopySource(native.GetInput())); err != nil {
		return fmt.Errorf("ETHHandler SyncGenesisHeader, contract params deserialize error: %v", err)
	}
	// Get current epoch operator
	operatorAddress, err := node_manager.GetCurConOperator(native)
	if err != nil {
		return fmt.Errorf("ETHHandler SyncGenesisHeader, get current consensus operator address error: %v", err)
	}

	//check witness, ensure the legitimate of the poly native service data
	err = utils.ValidateOwner(native, operatorAddress)
	if err != nil {
		return fmt.Errorf("ETHHandler SyncGenesisHeader, checkWitness error: %v", err)
	}
	//parse header data from native service input field
	header, err := getGenesisHeader(native.GetInput())
	if err != nil {
		return fmt.Errorf("ETHHandler SyncGenesisHeader: %s", err)
	}
	//look for genesis header from relay chain to make sure func SyncGenesisHeader is only called once
	headerStore, err := native.GetCacheDB().Get(utils.ConcatKey(utils.HeaderSyncContractAddress, []byte(scom.GENESIS_HEADER), utils.GetUint64Bytes(params.ChainID)))
	if err != nil {
		return fmt.Errorf("ETHHandler GetHeaderByHeight, get blockHashStore error: %v", err)
	}
	if headerStore != nil {
		return fmt.Errorf("ETHHandler GetHeaderByHeight, genesis header had been initialized")
	}
	//store the information of genesis header to poly chain
	err = putGenesisBlockHeader(native, header, params.ChainID)
	if err != nil {
		return fmt.Errorf("ETHHandler SyncGenesisHeader, put blockHeader error: %v", err)
	}

	return nil
}

func (this *ETHHandler) SyncBlockHeader(native *native.NativeService) error {
	headerParams := new(scom.SyncBlockHeaderParam)
	if err := headerParams.Deserialization(common.NewZeroCopySource(native.GetInput())); err != nil {
		return fmt.Errorf("SyncBlockHeader, contract params deserialize error: %v", err)
	}
	//check if chainid exist
	sideChain, err := side_chain_manager.GetSideChain(native, headerParams.ChainID)
	if err != nil {
		return fmt.Errorf("SyncBlockHeader, side_chain_manager.GetSideChain error: %v", err)
	}
	if sideChain == nil {
		return fmt.Errorf("SyncBlockHeader, side chain is not registered")
	}
	if sideChain.Name == "eth-ropsten" || sideChain.Name == "eth-main" {
		err = this.SyncPosBlockHeader(native, headerParams)
		return err
	}
	caches := NewCaches(3, native)
	for _, v := range headerParams.Headers {
		var header types.Header
		err := header.UnmarshalJSON(v)
		if err != nil {
			return fmt.Errorf("SyncBlockHeader, deserialize header err: %v", err)
		}
		headerHash := header.Hash()
		exist, err := IsHeaderExist(native, headerHash.Bytes(), headerParams.ChainID)
		if err != nil {
			return fmt.Errorf("SyncBlockHeader, check header exist err: %v", err)
		}
		if exist {
			log.Warnf("SyncBlockHeader, header has exist. Header: %s", string(v))
			continue
		}
		// get pre header
		parentHeader, parentDifficultySum, err := GetHeaderByHash(native, header.ParentHash.Bytes(), headerParams.ChainID)
		if err != nil {
			return fmt.Errorf("SyncBlockHeader, get the parent block failed. Error:%s, header: %s", err, string(v))
		}

		//verify difficulty
		var expected *big.Int
		if IsGrayGlacier(&header, sideChain.Name) {
			expected = makeDifficultyCalculator(big.NewInt(11_400_000))(header.Time, parentHeader)
		} else if isArrowGlacier(&header, sideChain.Name) {
			expected = makeDifficultyCalculator(big.NewInt(10_700_000))(header.Time, parentHeader)
		} else if isLondon(&header, sideChain.Name) {
			expected = makeDifficultyCalculator(big.NewInt(9700000))(header.Time, parentHeader)
		} else {
			expected = difficultyCalculator(new(big.Int).SetUint64(header.Time), parentHeader)
		}
		if expected.Cmp(header.Difficulty) != 0 {
			return fmt.Errorf("SyncBlockHeader, invalid difficulty: have %v, want %v, header: %s", header.Difficulty, expected, string(v))
		}

		// Verify that the gas limit is <= 2^63-1
		if header.GasLimit > uint64(0x7fffffffffffffff) {
			return fmt.Errorf("SyncBlockHeader, invalid gasLimit: have %v, max %v, header: %s", header.GasLimit, uint64(0x7fffffffffffffff), string(v))
		}
		// Verify that the gasUsed is <= gasLimit
		if header.GasUsed > header.GasLimit {
			return fmt.Errorf("SyncBlockHeader, invalid gasUsed: have %d, gasLimit %d, header: %s", header.GasUsed, header.GasLimit, string(v))
		}
		//London hard fork
		if isLondon(&header, sideChain.Name) {
			err = VerifyEip1559Header(parentHeader, &header, sideChain.Name)
		} else {
			err = VerifyGaslimit(parentHeader.GasLimit, header.GasLimit)
		}
		if err != nil {
			return fmt.Errorf("SyncBlockHeader, err:%v", err)
		}

		headerDifficultySum := new(big.Int).Add(header.Difficulty, parentDifficultySum)
		//block header storage store the mapping between block header hash and header data into poly
		err = putBlockHeader(native, header, headerDifficultySum, headerParams.ChainID)
		if err != nil {
			return fmt.Errorf("SyncGenesisHeader, put blockHeader error: %v, header: %s", err, string(v))
		}
		// get current header of main
		currentHeader, currentDifficultySum, err := GetCurrentHeader(native, headerParams.ChainID)
		if err != nil {
			return fmt.Errorf("SyncBlockHeader, get the current block failed. error:%s", err)
		}
		//make sure the block header synchronization is ordered
		if bytes.Equal(currentHeader.Hash().Bytes(), header.ParentHash.Bytes()) {
			//if header is just the next one
			err = appendHeader2Main(native, header.Number.Uint64(), headerHash, headerParams.ChainID)
			if err != nil {
				return err
			}
		} else {
			//The block to be synchronized belongs to another fork and has larger difficulty sum
			if headerDifficultySum.Cmp(currentDifficultySum) > 0 {
				// reconstruct the chain following the new fork
				err := RestructChain(native, currentHeader, &header, headerParams.ChainID)
				if err != nil {
					return err
				}
			}
		}
	}
	caches.deleteCaches()
	return nil
}

func (this *ETHHandler) SyncPosBlockHeader(native *native.NativeService, headerParams *scom.SyncBlockHeaderParam) error {
	// verifyHeader checks whether a header conforms to the consensus rules of the
	// stock Ethereum consensus engine. The difference between the beacon and classic is
	// (a) The following fields are expected to be constants:
	//     - difficulty is expected to be 0
	// 	   - nonce is expected to be 0
	//     - unclehash is expected to be Hash(emptyHeader)
	//     to be the desired constants
	// (b) the timestamp is not verified anymore
	// (c) the extradata is limited to 32 bytes
	caches := NewCaches(3, native)
	for _, v := range headerParams.Headers {
		var header types.Header
		err := header.UnmarshalJSON(v)
		if err != nil {
			return fmt.Errorf("SyncBlockHeader, deserialize header err: %v", err)
		}
		headerHash := header.Hash()
		exist, err := IsHeaderExist(native, headerHash.Bytes(), headerParams.ChainID)
		if err != nil {
			return fmt.Errorf("SyncBlockHeader, check header exist err: %v", err)
		}
		if exist == true {
			log.Warnf("SyncBlockHeader, header has exist. Header: %s", string(v))
			continue
		}
		// get pre header
		parent, _, err := GetHeaderByHash(native, header.ParentHash.Bytes(), headerParams.ChainID)
		if err != nil {
			return fmt.Errorf("SyncBlockHeader, get the parent block failed. Error:%s, header: %s", err, string(v))
		}
		// Ensure that the header's extra-data section is of a reasonable size
		if len(header.Extra) > 32 {
			return fmt.Errorf("SyncBlockHeader, extra-data longer than 32 bytes (%d)", len(header.Extra))
		}
		// Verify the seal parts. Ensure the nonce and uncle hash are the expected value.
		if header.Nonce != beaconNonce {
			return fmt.Errorf("SyncBlockHeader, invalid nonce")
		}
		if header.UncleHash != types.EmptyUncleHash {
			return fmt.Errorf("SyncBlockHeader, invalid uncle hash")
		}
		// Verify the block's difficulty to ensure it's the default constant
		if beaconDifficulty.Cmp(header.Difficulty) != 0 {
			return fmt.Errorf("invalid difficulty: have %v, want %v", header.Difficulty, beaconDifficulty)
		}
		// Verify that the gas limit is <= 2^63-1
		if header.GasLimit > params.MaxGasLimit {
			return fmt.Errorf("invalid gasLimit: have %v, max %v", header.GasLimit, params.MaxGasLimit)
		}
		// Verify that the gasUsed is <= gasLimit
		if header.GasUsed > header.GasLimit {
			return fmt.Errorf("invalid gasUsed: have %d, gasLimit %d", header.GasUsed, header.GasLimit)
		}
		// Verify that the block number is parent's +1
		if diff := new(big.Int).Sub(header.Number, parent.Number); diff.Cmp(ethcommon.Big1) != 0 {
			return consensus.ErrInvalidNumber
		}
		// Verify the header's EIP-1559 attributes.
		err = VerifyEip1559Header(parent, &header, "")
		if err != nil {
			return fmt.Errorf("SyncBlockHeader, err:%v", err)
		}

		err = putBlockHeader(native, header, big.NewInt(0), headerParams.ChainID)
		if err != nil {
			return fmt.Errorf("SyncGenesisHeader, put blockHeader error: %v, header: %s", err, string(v))
		}
		// get current header of main
		currentHeader, _, err := GetCurrentHeader(native, headerParams.ChainID)
		if err != nil {
			return fmt.Errorf("SyncBlockHeader, get the current block failed. error:%s", err)
		}
		//make sure the block header synchronization is ordered
		if bytes.Equal(currentHeader.Hash().Bytes(), header.ParentHash.Bytes()) {
			//if header is just the next one
			err = appendHeader2Main(native, header.Number.Uint64(), headerHash, headerParams.ChainID)
			if err != nil {
				return err
			}
		} else {
			// reconstruct the chain following the new fork
			err := RestructChain(native, currentHeader, &header, headerParams.ChainID)
			if err != nil {
				return err
			}
		}
	}
	caches.deleteCaches()
	return nil
}

func (this *ETHHandler) SyncCrossChainMsg(native *native.NativeService) error {
	return nil
}

func getGenesisHeader(input []byte) (types.Header, error) {
	params := new(scom.SyncGenesisHeaderParam)
	if err := params.Deserialization(common.NewZeroCopySource(input)); err != nil {
		return types.Header{}, fmt.Errorf("getGenesisHeader, contract params deserialize error: %v", err)
	}
	var header types.Header
	err := header.UnmarshalJSON(params.GenesisHeader)
	if err != nil {
		return types.Header{}, fmt.Errorf("getGenesisHeader, deserialize header err: %v", err)
	}
	return header, nil
}

func difficultyCalculator(time *big.Int, parent *types.Header) *big.Int {
	// https://github.com/ethereum/EIPs/issues/100.
	// algorithm:
	// diff = (parent_diff +
	//         (parent_diff / 2048 * max((2 if len(parent.uncles) else 1) - ((timestamp - parent.timestamp) // 9), -99))
	//        ) + 2^(periodCount / 10000 - 2)
	// diff = parent_diff + diff_adjust + diff_bomb

	//difficulty adjustment
	//(parent_diff / 2048 * max((2 if len(parent.uncles) else 1) - ((timestamp - parent.timestamp) // 9), -99))
	x := new(big.Int).Sub(time, new(big.Int).SetUint64(parent.Time))
	x.Div(x, BIG_9)

	if parent.UncleHash == types.EmptyUncleHash {
		x.Sub(BIG_1, x)
	} else {
		x.Sub(BIG_2, x)
	}

	if x.Cmp(BIG_MINUS_99) < 0 {
		x.Set(BIG_MINUS_99)
	}

	y := new(big.Int).Div(parent.Difficulty, BLOCK_DIFF_FACTOR)
	x.Mul(y, x)
	x.Add(parent.Difficulty, x)

	if x.Cmp(params.MinimumDifficulty) < 0 {
		x.Set(params.MinimumDifficulty)
	}

	//difficulty bomb
	//https://eips.ethereum.org/EIPS/eip-1234
	fakeBlockNumber := new(big.Int)
	if parent.Number.Cmp(BOMB_DELAY) >= 0 {
		fakeBlockNumber = fakeBlockNumber.Sub(parent.Number, BOMB_DELAY)
	}

	periodCount := fakeBlockNumber
	periodCount.Div(periodCount, DIFF_PERIOD)

	if periodCount.Cmp(BIG_1) > 0 {
		y.Sub(periodCount, BIG_2)
		y.Exp(BIG_2, y, nil)
		x.Add(x, y)
	}
	return x
}

func (this *ETHHandler) verifyHeader(header *types.Header, caches *Caches) error {
	// try to verfify header
	number := header.Number.Uint64()
	size := datasetSize(number)
	headerHash := HashHeader(header).Bytes()
	nonce := header.Nonce.Uint64()
	// get seed and seed head
	seed := make([]byte, 40)
	copy(seed, headerHash)
	binary.LittleEndian.PutUint64(seed[32:], nonce)
	seed = crypto.Keccak512(seed)
	// get mix
	mix := make([]uint32, mixBytes/4)
	for i := 0; i < len(mix); i++ {
		mix[i] = binary.LittleEndian.Uint32(seed[i%16*4:])
	}
	// get cache
	cache := caches.getCache(number)
	if len(cache) <= 1 {
		return fmt.Errorf("cache of proof-of-work is not generated!")
	}
	// get new mix with DAG data
	rows := uint32(size / mixBytes)
	temp := make([]uint32, len(mix))
	seedHead := binary.LittleEndian.Uint32(seed)
	for i := 0; i < loopAccesses; i++ {
		parent := fnv(uint32(i)^seedHead, mix[i%len(mix)]) % rows
		for j := uint32(0); j < mixBytes/hashBytes; j++ {
			xx := lookup(cache, 2*parent+j)
			copy(temp[j*hashWords:], xx)
		}
		fnvHash(mix, temp)
	}
	// get new mix by compress
	for i := 0; i < len(mix); i += 4 {
		mix[i/4] = fnv(fnv(fnv(mix[i], mix[i+1]), mix[i+2]), mix[i+3])
	}
	mix = mix[:len(mix)/4]
	// get digest by compressed mix
	digest := make([]byte, ethcommon.HashLength)
	for i, val := range mix {
		binary.LittleEndian.PutUint32(digest[i*4:], val)
	}
	// get header result hash
	result := crypto.Keccak256(append(seed, digest...))
	// Verify the calculated digest against the ones provided in the header
	if !bytes.Equal(header.MixDigest[:], digest) {
		return fmt.Errorf("invalid mix digest!")
	}
	// compare result hash with target hash
	target := new(big.Int).Div(two256, header.Difficulty)
	if new(big.Int).SetBytes(result).Cmp(target) > 0 {
		return fmt.Errorf("invalid proof-of-work!")
	}
	return nil
}

func HashHeader(header *types.Header) (hash ethcommon.Hash) {
	hasher := sha3.NewLegacyKeccak256()
	enc := []interface{}{
		header.ParentHash,
		header.UncleHash,
		header.Coinbase,
		header.Root,
		header.TxHash,
		header.ReceiptHash,
		header.Bloom,
		header.Difficulty,
		header.Number,
		header.GasLimit,
		header.GasUsed,
		header.Time,
		header.Extra,
	}
	if header.BaseFee != nil {
		enc = append(enc, header.BaseFee)
	}
	rlp.Encode(hasher, enc)
	hasher.Sum(hash[:0])
	return hash
}

type hasher func(dest []byte, data []byte)

func makeHasher(h hash.Hash) hasher {
	// sha3.state supports Read to get the sum, use it to avoid the overhead of Sum.
	// Read alters the state but we reset the hash before every operation.
	type readerHash interface {
		hash.Hash
		Read([]byte) (int, error)
	}
	rh, ok := h.(readerHash)
	if !ok {
		panic("can't find Read method on hash")
	}
	outputLen := rh.Size()
	return func(dest []byte, data []byte) {
		rh.Reset()
		rh.Write(data)
		rh.Read(dest[:outputLen])
	}
}

func seedHash(block uint64) []byte {
	seed := make([]byte, 32)
	if block < epochLength {
		return seed
	}
	keccak256 := makeHasher(sha3.NewLegacyKeccak256())
	for i := 0; i < int(block/epochLength); i++ {
		keccak256(seed, seed)
	}
	return seed
}
