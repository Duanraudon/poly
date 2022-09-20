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
package eth_clique

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"hash"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/polynetwork/poly/common/log"
	"github.com/polynetwork/poly/native/service/governance/node_manager"
	"golang.org/x/crypto/sha3"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/polynetwork/poly/common"
	"github.com/polynetwork/poly/native"
	scom "github.com/polynetwork/poly/native/service/header_sync/common"
	"github.com/polynetwork/poly/native/service/utils"
)

// Various error messages to mark blocks invalid. These should be private to
// prevent engine specific errors from being referenced in the remainder of the
// codebase, inherently breaking if the engine is swapped out. Please put common
// error types into the consensus package.
var (
	// errUnknownBlock is returned when the list of signers is requested for a block
	// that is not part of the local blockchain.
	errUnknownBlock = errors.New("unknown block")

	// errInvalidCheckpointBeneficiary is returned if a checkpoint/epoch transition
	// block has a beneficiary set to non-zeroes.
	errInvalidCheckpointBeneficiary = errors.New("beneficiary in checkpoint block non-zero")

	// errInvalidVote is returned if a nonce value is something else that the two
	// allowed constants of 0x00..0 or 0xff..f.
	errInvalidVote = errors.New("vote nonce not 0x00..0 or 0xff..f")

	// errInvalidCheckpointVote is returned if a checkpoint/epoch transition block
	// has a vote nonce set to non-zeroes.
	errInvalidCheckpointVote = errors.New("vote nonce in checkpoint block non-zero")

	// errMissingVanity is returned if a block's extra-data section is shorter than
	// 32 bytes, which is required to store the signer vanity.
	errMissingVanity = errors.New("extra-data 32 byte vanity prefix missing")

	// errMissingSignature is returned if a block's extra-data section doesn't seem
	// to contain a 65 byte secp256k1 signature.
	errMissingSignature = errors.New("extra-data 65 byte signature suffix missing")

	// errExtraSigners is returned if non-checkpoint block contain signer data in
	// their extra-data fields.
	errExtraSigners = errors.New("non-checkpoint block contains extra signer list")

	// errInvalidCheckpointSigners is returned if a checkpoint block contains an
	// invalid list of signers (i.e. non divisible by 20 bytes).
	errInvalidCheckpointSigners = errors.New("invalid signer list on checkpoint block")

	// errMismatchingCheckpointSigners is returned if a checkpoint block contains a
	// list of signers different than the one the local node calculated.
	errMismatchingCheckpointSigners = errors.New("mismatching signer list on checkpoint block")

	// errInvalidMixDigest is returned if a block's mix digest is non-zero.
	errInvalidMixDigest = errors.New("non-zero mix digest")

	// errInvalidUncleHash is returned if a block contains an non-empty uncle list.
	errInvalidUncleHash = errors.New("non empty uncle hash")

	// errInvalidDifficulty is returned if the difficulty of a block neither 1 or 2.
	errInvalidDifficulty = errors.New("invalid difficulty")

	// errWrongDifficulty is returned if the difficulty of a block doesn't match the
	// turn of the signer.
	errWrongDifficulty = errors.New("wrong difficulty")

	// errInvalidTimestamp is returned if the timestamp of a block is lower than
	// the previous block's timestamp + the minimum block period.
	errInvalidTimestamp = errors.New("invalid timestamp")

	// errInvalidVotingChain is returned if an authorization list is attempted to
	// be modified via out-of-range or non-contiguous headers.
	errInvalidVotingChain = errors.New("invalid voting chain")

	// errUnauthorizedSigner is returned if a header is signed by a non-authorized entity.
	errUnauthorizedSigner = errors.New("unauthorized signer")

	// errRecentlySigned is returned if a header is signed by an authorized entity
	// that already signed a header recently, thus is temporarily not allowed to.
	errRecentlySigned = errors.New("recently signed")
)

// Clique proof-of-authority protocol constants.
var (
	extraVanity = 32                     // Fixed number of extra-data prefix bytes reserved for signer vanity
	extraSeal   = crypto.SignatureLength // Fixed number of extra-data suffix bytes reserved for signer seal

	nonceAuthVote = hexutil.MustDecode("0xffffffffffffffff") // Magic nonce number to vote on adding a new signer
	nonceDropVote = hexutil.MustDecode("0x0000000000000000") // Magic nonce number to vote on removing a signer.

	uncleHash = types.CalcUncleHash(nil) // Always Keccak256(RLP([])) as uncles are meaningless outside of PoW.

	diffInTurn = big.NewInt(2) // Block difficulty for in-turn signatures
	diffNoTurn = big.NewInt(1) // Block difficulty for out-of-turn signatures
)

const Epoch = 30000

type ETHCliHander struct {
}

func NewETHCliHander() *ETHCliHander {
	return &ETHCliHander{}
}

func (this *ETHCliHander) SyncGenesisHeader(native *native.NativeService) error {
	params := new(scom.SyncGenesisHeaderParam)
	if err := params.Deserialization(common.NewZeroCopySource(native.GetInput())); err != nil {
		return fmt.Errorf("ETHCliHander SyncGenesisHeader, contract params deserialize error: %v", err)
	}
	// Get current epoch operator
	operatorAddress, err := node_manager.GetCurConOperator(native)
	if err != nil {
		return fmt.Errorf("ETHCliHander SyncGenesisHeader, get current consensus operator address error: %v", err)
	}

	//check witness
	err = utils.ValidateOwner(native, operatorAddress)
	if err != nil {
		return fmt.Errorf("ETHCliHander SyncGenesisHeader, checkWitness error: %v", err)
	}

	header, err := getGenesisHeader(native.GetInput())
	if err != nil {
		return fmt.Errorf("ETHCliHander SyncGenesisHeader: %s", err)
	}

	headerStore, err := native.GetCacheDB().Get(utils.ConcatKey(utils.HeaderSyncContractAddress, []byte(scom.GENESIS_HEADER), utils.GetUint64Bytes(params.ChainID)))
	if err != nil {
		return fmt.Errorf("ETHCliHander GetHeaderByHeight, get blockHashStore error: %v", err)
	}
	if headerStore != nil {
		return fmt.Errorf("ETHCliHander GetHeaderByHeight, genesis header had been initialized")
	}
	jHeader, _ := json.Marshal(header)
	log.Infof("GenesisHeader: %s", string(jHeader))
	log.Infof("GenesisHeaderHash: %s", header.Hash().String())
	//block header storage
	err = putGenesisBlockHeader(native, header, params.ChainID)
	if err != nil {
		return fmt.Errorf("ETHCliHander SyncGenesisHeader, put blockHeader error: %v", err)
	}

	return nil
}

func (this *ETHCliHander) SyncBlockHeader(native *native.NativeService) error {
	headerParams := new(scom.SyncBlockHeaderParam)
	if err := headerParams.Deserialization(common.NewZeroCopySource(native.GetInput())); err != nil {
		return fmt.Errorf("SyncBlockHeader, contract params deserialize error: %v", err)
	}
	caches := NewCaches(3, native)
	for _, v := range headerParams.Headers {
		var header types.Header
		err := json.Unmarshal(v, &header)
		if err != nil {
			return fmt.Errorf("SyncBlockHeader, deserialize header err: %v", err)
		}
		exist, err := IsHeaderExist(native, header.Hash().Bytes(), headerParams.ChainID)
		if err != nil {
			return fmt.Errorf("SyncBlockHeader, check header exist err: %v", err)
		}
		if exist == true {
			log.Warnf("SyncBlockHeader, header has exist. Header: %s", string(v))
			continue
		}

		if header.Number == nil {
			return errUnknownBlock
		}
		number := header.Number.Uint64()

		// Don't waste time checking blocks from the future
		if header.Time > uint64(time.Now().Unix()) {
			return consensus.ErrFutureBlock
		}
		// Checkpoint blocks need to enforce zero beneficiary
		//checkpoint := (number % Epoch) == 0
		//if checkpoint && header.Coinbase != (ethcommon.Address{}) {
		//	return errInvalidCheckpointBeneficiary
		//}
		// Nonces must be 0x00..0 or 0xff..f, zeroes enforced on checkpoints
		if !bytes.Equal(header.Nonce[:], nonceAuthVote) && !bytes.Equal(header.Nonce[:], nonceDropVote) {
			return errInvalidVote
		}
		//if checkpoint && !bytes.Equal(header.Nonce[:], nonceDropVote) {
		//	return errInvalidCheckpointVote
		//}
		// Check that the extra-data contains both the vanity and signature
		if len(header.Extra) < extraVanity {
			return errMissingVanity
		}
		if len(header.Extra) < extraVanity+extraSeal {
			return errMissingSignature
		}
		// Ensure that the extra-data contains a signer list on checkpoint, but none otherwise
		//signersBytes := len(header.Extra) - extraVanity - extraSeal
		//if !checkpoint && signersBytes != 0 {
		//	return errExtraSigners
		//}
		//if checkpoint && signersBytes%ethcommon.AddressLength != 0 {
		//	return errInvalidCheckpointSigners
		//}
		// Ensure that the mix digest is zero as we don't have fork protection currently
		if header.MixDigest != (ethcommon.Hash{}) {
			return errInvalidMixDigest
		}
		// Ensure that the block doesn't contain any uncles which are meaningless in PoA
		if header.UncleHash != uncleHash {
			return errInvalidUncleHash
		}
		// Ensure that the block's difficulty is meaningful (may not be correct at this point)
		if number > 0 {
			if header.Difficulty == nil || (header.Difficulty.Cmp(diffInTurn) != 0 && header.Difficulty.Cmp(diffNoTurn) != 0) {
				return errInvalidDifficulty
			}
		}
		// Verify that the gas limit is <= 2^63-1
		if header.GasLimit > params.MaxGasLimit {
			return fmt.Errorf("invalid gasLimit: have %v, max %v", header.GasLimit, params.MaxGasLimit)
		}

		parentHeader, parentDifficultySum, err := GetHeaderByHeight(native, header.Number.Uint64()-1, headerParams.ChainID)
		if err != nil {
			return fmt.Errorf("SyncBlockHeader, get the current block failed. error:%s", err)
		}

		//if isLondon(&header) {
		//	err = VerifyEip1559Header(parentHeader, &header)
		//} else {
		err = VerifyGaslimit(parentHeader.GasLimit, header.GasLimit)
		//}
		if err != nil {
			return fmt.Errorf("SyncBlockHeader, err:%v", err)
		}

		headerDifficulty := header.Difficulty
		headerDifficultySum := new(big.Int).Add(headerDifficulty, parentDifficultySum)
		err = putBlockHeader(native, header, headerDifficultySum, headerParams.ChainID)
		if err != nil {
			return fmt.Errorf("SyncGenesisHeader, put blockHeader error: %v, header: %s", err, string(v))
		}

		// get current header of main
		currentHeader, currentDifficultySum, err := GetCurrentHeader(native, headerParams.ChainID)
		if err != nil {
			return fmt.Errorf("SyncBlockHeader, get the current block failed. error:%s", err)
		}

		// todo: The header hash here does not match the actual block hash
		if parentHeader.Number.Int64() == header.Number.Int64()-1 {
			err := appendHeader2Main(native, header.Number.Uint64(), header.Hash(), headerParams.ChainID)
			if err != nil {
				return err
			}
		} else {
			if headerDifficultySum.Cmp(currentDifficultySum) > 0 {
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

func (this *ETHCliHander) SyncCrossChainMsg(native *native.NativeService) error {
	return nil
}

func getGenesisHeader(input []byte) (types.Header, error) {
	params := new(scom.SyncGenesisHeaderParam)
	if err := params.Deserialization(common.NewZeroCopySource(input)); err != nil {
		return types.Header{}, fmt.Errorf("getGenesisHeader, contract params deserialize error: %v", err)
	}
	var header types.Header
	err := json.Unmarshal(params.GenesisHeader, &header)
	if err != nil {
		return types.Header{}, fmt.Errorf("getGenesisHeader, deserialize header err: %v", err)
	}
	return header, nil
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
