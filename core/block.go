// Definition of block

package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"log"
	"math/big"
	"time"
)

// The definition of blockheader
type BlockHeader struct {
	ParentBlockHash []byte
	StateRoot       []byte
	TxRoot          []byte
	Number          uint64
	Time            time.Time
	Miner           uint64

	//add
	ParentHash  common.Hash    `json:"parentHash" gencodec:"required"`
	Coinbase    common.Address `json:"miner"`
	Root        common.Hash    `json:"stateRoot"        gencodec:"required"`
	TxHash      common.Hash    `json:"transactionsRoot" gencodec:"required"`
	ReceiptHash common.Hash    `json:"receiptsRoot"     gencodec:"required"`
	//Bloom       Bloom          `json:"logsBloom"        gencodec:"required"`
	Difficulty *big.Int `json:"difficulty"       gencodec:"required"`
	//Number      *big.Int       `json:"number"           gencodec:"required"`
	GasLimit uint64 `json:"gasLimit"         gencodec:"required"`
	GasUsed  uint64 `json:"gasUsed"          gencodec:"required"`
	//Time      uint64      `json:"timestamp"        gencodec:"required"`
	Extra     []byte      `json:"extraData"        gencodec:"required"`
	MixDigest common.Hash `json:"mixHash"`
	BaseFee   *big.Int    `json:"baseFeePerGas" rlp:"optional"`
}

// Encode blockHeader for storing further
func (bh *BlockHeader) Encode() []byte {
	var buff bytes.Buffer
	enc := gob.NewEncoder(&buff)
	err := enc.Encode(bh)
	if err != nil {
		log.Panic(err)
	}
	return buff.Bytes()
}

// Decode blockHeader
func DecodeBH(b []byte) *BlockHeader {
	var blockHeader BlockHeader

	decoder := gob.NewDecoder(bytes.NewReader(b))
	err := decoder.Decode(&blockHeader)
	if err != nil {
		log.Panic(err)
	}

	return &blockHeader
}

// Hash the blockHeader
func (bh *BlockHeader) Hash() []byte {
	hash := sha256.Sum256(bh.Encode())
	return hash[:]
}

func (bh *BlockHeader) PrintBlockHeader() string {
	vals := []interface{}{
		hex.EncodeToString(bh.ParentBlockHash),
		hex.EncodeToString(bh.StateRoot),
		hex.EncodeToString(bh.TxRoot),
		bh.Number,
		bh.Time,
	}
	res := fmt.Sprintf("%v\n", vals)
	return res
}

// The definition of block
type Block struct {
	Header *BlockHeader
	Body   []*Transaction
	Hash   []byte
}

func NewBlock(bh *BlockHeader, bb []*Transaction) *Block {
	return &Block{Header: bh, Body: bb}
}

func (b *Block) PrintBlock() string {
	vals := []interface{}{
		b.Header.Number,
		b.Hash,
	}
	res := fmt.Sprintf("%v\n", vals)
	fmt.Println(res)
	return res
}

// Encode Block for storing
func (b *Block) Encode() []byte {
	var buff bytes.Buffer
	enc := gob.NewEncoder(&buff)
	err := enc.Encode(b)
	if err != nil {
		log.Panic(err)
	}
	return buff.Bytes()
}

// Decode Block
func DecodeB(b []byte) *Block {
	var block Block

	decoder := gob.NewDecoder(bytes.NewReader(b))
	err := decoder.Decode(&block)
	if err != nil {
		log.Panic(err)
	}

	return &block
}
