package core

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"math/big"
)

type Message struct {
	To            common.Address
	From          common.Address
	Nonce         uint64
	Value         *big.Int
	GasLimit      uint64
	GasPrice      *big.Int
	GasFeeCap     *big.Int
	GasTipCap     *big.Int
	Data          []byte
	AccessList    types.AccessList
	BlobGasFeeCap *big.Int
	BlobHashes    []common.Hash

	// When SkipNonceChecks is true, the message nonce is not checked against the
	// account nonce in state.
	// This field will be set to true for operations like RPC eth_call.
	SkipNonceChecks bool

	// When SkipFromEOACheck is true, the message sender is not checked to be an EOA.
	SkipFromEOACheck bool
}
