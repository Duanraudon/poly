package eos

import (
	"crypto/sha256"
	"fmt"

	"github.com/polynetwork/poly/common"
	"github.com/polynetwork/poly/common/log"
)

type EOSProof struct {
	path [][]byte
	leaf []byte
}

var (
	LeftSign  uint64 = 0xFFFFFFFFFFFFFF7F
	RightSign uint64 = 0x0000000000000080
)

func (this *EOSProof) Serialization(sink *common.ZeroCopySink) {
	sink.WriteBytes(this.leaf)
	// fmt.Printf("Write leaf len is:%v\n", len(this.leaf))
	for i := 0; i < len(this.path); i++ {
		sink.WriteBytes(this.path[i])
		// fmt.Printf("Write path %d is:%v\n", i, len(this.path[i]))
	}
}

func (this *EOSProof) Deserialization(data []byte) error {

	source := common.NewZeroCopySource(data)

	n := source.Len()
	// fmt.Printf("source len is:%d", n)
	if (n % 32) != 0 {
		return fmt.Errorf("Deserialization error : len is illegal")
	}
	var path [][]byte
	var leaf []byte
	leaf, eof := source.NextBytes(32)
	if eof {
		return fmt.Errorf("Waiting deserialize leaf error")
	}

	for i := 0; i < int(n/32)-1; i++ {
		pa, eof := source.NextBytes(32)
		fmt.Printf("the source the %d pa len is: %d\n", i+1, source.Len())
		if eof {
			return fmt.Errorf("Waiting deserialize %d path error", i)
		}
		path = append(path, pa)
	}

	this.leaf = leaf
	this.path = path
	return nil
}

func VerifyProof1(path [][]byte, leaf []byte) []byte {
	tempLeaf := make([]byte, 32)
	//tempPath := make([]byte, 32)
	copy(tempLeaf, leaf)
	log.Errorf("tempLeaf,%v", tempLeaf)
	for _, pa := range path {
		log.Errorf("pa,%v", pa)
		//tempPath = pa
		//log.Errorf("tempPath,%v", pa)
		if JudgeLeft(pa) {
			log.Error("VerifyProof2")
			tempLeaf = SignToRight(tempLeaf)
			tempLeaf = CalculateNodeHash(pa, tempLeaf)
		} else {
			log.Error("VerifyProof3")
			tempLeaf = SignToLeft(tempLeaf)
			tempLeaf = CalculateNodeHash(tempLeaf, pa)
		}
	}
	return tempLeaf
}

func VerifyProof(path [][]byte, leaf []byte) []byte {
	tempLeaf := make([]byte, 32)
	copy(tempLeaf, leaf)
	log.Errorf("tempLeaf,%v", tempLeaf)
	log.Errorf("path,%v", path)
	for _, pa := range path {
		var tempPath []byte
		tempPath = pa
		log.Errorf("pa,%v", pa)
		if JudgeLeft(tempPath) {

			log.Error("VerifyProof2")
			tempLeaf = SignToRight(tempLeaf)
			tempLeaf = CalculateNodeHash(tempPath, tempLeaf)
		} else {
			log.Error("VerifyProof3")
			tempLeaf = SignToLeft(tempLeaf)
			tempLeaf = CalculateNodeHash(tempLeaf, tempPath)
		}
	}
	return tempLeaf
}

func JudgeLeft(left []byte) bool {
	return (left[0] & byte(RightSign)) == 0
}
func Judgeright(right []byte) bool {
	return (right[0] & byte(RightSign)) != 0
}

/*
左节点标记
*/
func SignToLeft(node []byte) []byte {
	left := node
	left[0] &= byte(LeftSign)

	return left
}

/*
	右节点标记
*/
func SignToRight(node []byte) []byte {
	right := node
	right[0] |= byte(RightSign)
	return right
}

func CalculateHash(hash []byte) []byte {
	h := sha256.New()
	_, _ = h.Write(hash)
	return h.Sum(nil)
}

// func CalculateNodeHash(left, right []byte) []byte {
// 	temp := append(left, right...)
// 	return CalculateHash(temp)
// }

func CalculateNodeHash(left, right []byte) []byte {
	var temp []byte
	temp = append(temp, left...)
	temp = append(temp, right...)
	return CalculateHash(temp)
}
