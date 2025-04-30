// Definition of transaction

package core

import (
	"blockEmulator/utils"
	"blockEmulator/vm/state"
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type Transaction struct {
	Sender    utils.Address
	Recipient utils.Address
	Nonce     uint64
	Signature []byte // not implemented now.
	Value     *big.Int
	Txtype    string
	Fee       *big.Int
	TxHash    []byte

	Time time.Time // the time adding in pool

	// used in transaction relaying
	Relayed bool
	// used in broker, if the tx is not a broker1 or broker2 tx, these values should be empty.
	HasBroker           bool
	SenderIsBroker      bool
	IsAllocatedSender   bool
	IsAllocatedRecipent bool
	OriginalSender      utils.Address
	FinalRecipient      utils.Address
	RawTxHash           []byte
	Isbrokertx1         bool
	Isbrokertx2         bool

	//Isimportant bool

	Gas              uint64
	GasPrice         *big.Int
	To               common.Address
	From             common.Address
	Data             []byte
	IsBrokerContract bool

	Log  []state.Wrapper
	UUID string
}

func (tx *Transaction) PrintTx() string {
	vals := []interface{}{
		tx.Sender[:],
		tx.Recipient[:],
		tx.Value,
		string(tx.TxHash[:]),
	}
	res := fmt.Sprintf("%v\n", vals)
	return res
}

// Encode transaction for storing
func (tx *Transaction) Encode() []byte {
	var buff bytes.Buffer

	enc := gob.NewEncoder(&buff)
	err := enc.Encode(tx)
	if err != nil {
		log.Panic(err)
	}

	return buff.Bytes()
}

// Decode transaction
func DecodeTx(to_decode []byte) *Transaction {
	var tx Transaction

	decoder := gob.NewDecoder(bytes.NewReader(to_decode))
	err := decoder.Decode(&tx)
	if err != nil {
		log.Panic(err)
	}

	return &tx
}

// new a transaction
func NewTransaction(sender, recipient string, value *big.Int, nonce uint64, fee *big.Int, tx_type string) *Transaction {
	tx := &Transaction{
		Sender:    sender,
		Recipient: recipient,
		Value:     value,
		Txtype:    tx_type,
		Nonce:     nonce,
		Fee:       fee,
	}

	hash := sha256.Sum256(tx.Encode())
	tx.TxHash = hash[:]
	tx.Relayed = false
	tx.FinalRecipient = ""
	tx.OriginalSender = ""
	tx.RawTxHash = nil
	tx.HasBroker = false
	tx.SenderIsBroker = false
	tx.IsAllocatedSender = false
	tx.IsAllocatedRecipent = false
	return tx
}
