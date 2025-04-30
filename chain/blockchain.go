// Here the blockchain structrue is defined
// each node in this system will maintain a blockchain object.

package chain

import (
	"blockEmulator/core"
	"blockEmulator/global"
	"blockEmulator/global2"
	"blockEmulator/global3"
	"blockEmulator/params"
	"blockEmulator/storage"
	"blockEmulator/utils"
	"blockEmulator/vm"
	"blockEmulator/vm/state"
	"blockEmulator/vm/tracing"
	"blockEmulator/vm/trie"
	"blockEmulator/vm/trie/trienode"
	"blockEmulator/vm/triedb"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/holiman/uint256"
)

type BlockChain struct {
	Db ethdb.Database // the leveldb database to store in the disk, for status trie

	Triedb  *triedb.Database // the trie database which helps to store the status trie
	Statedb *state.CachingDB

	ChainConfig  *params.ChainConfig // the chain configuration, which can help to identify the chain
	CurrentBlock *core.Block         // the top block in this blockchain
	Storage      *storage.Storage    // Storage is the bolt-db to store the blocks
	Txpool       *core.TxPool        // the transaction pool
	Iptable      map[uint64]map[uint64]string
}

// Get the transaction root, this root can be used to check the transactions
func GetTxTreeRoot(txs []*core.Transaction) []byte {
	// use a memory trie database to do this, instead of disk database
	//triedb := trie.NewDatabase(rawdb.NewMemoryDatabase())
	triedb := triedb.NewDatabase(rawdb.NewMemoryDatabase(), nil)

	transactionTree := trie.NewEmpty(triedb)
	for _, tx := range txs {
		transactionTree.Update(tx.TxHash, tx.Encode())
	}
	return transactionTree.Hash().Bytes()
}

// Write Partition Map
func (bc *BlockChain) Update_PartitionMap(key string, val uint64) {
	global.Pmlock.Lock()
	defer global.Pmlock.Unlock()
	//bc.PartitionMap[key] = val
	global.PartitionMap[key] = val
}

// Get parition (if not exist, return default)
func (bc *BlockChain) Get_PartitionMap(key string) uint64 {
	global.Pmlock.RLock()
	defer global.Pmlock.RUnlock()
	if _, ok := global.PartitionMap[key]; !ok {
		return uint64(utils.Addr2Shard(key))
	}
	return global.PartitionMap[key]
}

// Send a transaction to the pool (need to decide which pool should be sended)
func (bc *BlockChain) SendTx2Pool(txs []*core.Transaction) {
	bc.Txpool.AddTxs2Pool(txs)
}

func (bc *BlockChain) GetUpdateStatusTrie(txs []*core.Transaction, blockHeader *core.BlockHeader, parentBlock *core.Block) common.Hash {

	var (
		gp             = new(vm.GasPool).AddGas(blockHeader.GasLimit)
		statedb, _, sn = state.New(global2.Root, bc.Statedb)
	)
	for idx, tx := range txs {

		UUID := tx.UUID

		FromStr := hex.EncodeToString(tx.From[:])
		ToStr := hex.EncodeToString(tx.To[:])

		for {
			global.GLobalLock.Lock()
			if _, ok1 := global.GlobalLockMap[FromStr]; ok1 {
				global.GLobalLock.Unlock()
				continue
			}

			if _, ok2 := global.GlobalLockMap[ToStr]; ok2 {
				global.GLobalLock.Unlock()
				continue
			}
			global.GlobalLockMap[FromStr] = true
			global.GlobalLockMap[ToStr] = true

			global.GLobalLock.Unlock()
			break
		}

		ExeTx(tx, statedb, blockHeader, bc, idx, gp, UUID)

		global.GLobalLock.Lock()
		delete(global.GlobalLockMap, FromStr)
		delete(global.GlobalLockMap, ToStr)
		global.GLobalLock.Unlock()

	}

	var hash common.Hash
	if sn == "new" {

		var E1 error
		global2.Root, E1 = statedb.Commit(0, false)
		if E1 != nil {
			fmt.Println(E1)
		}
		err1 := bc.Triedb.Commit(global2.Root, false)
		if err1 != nil {
			fmt.Println(err1)
		}
		global3.Lock.Lock()
		global3.GlobalStateDB = nil
		global3.Lock.Unlock()
	}

	return hash
}

func ExeTx(tx *core.Transaction, statedb *state.StateDB, blockHeader *core.BlockHeader, bc *BlockChain, idx int, gp *vm.GasPool, UUID string) {
	//TODO 普通转账交易添加txinfo

	if tx.IsAllocatedSender || tx.IsAllocatedRecipent {
		if tx.IsAllocatedSender {
			value, _ := uint256.FromBig(tx.Value)
			s, _ := hex.DecodeString(tx.Sender)
			statedb.SubBalance(common.Address(s), value, tracing.BalanceSubByXBZ)

		}
		if tx.IsAllocatedRecipent {

			value, _ := uint256.FromBig(tx.Value)
			s, _ := hex.DecodeString(tx.Recipient)
			statedb.AddBalance(common.Address(s), value, tracing.BalanceAddByXBZ)

		}
		return
	}
	if !tx.Relayed && (bc.Get_PartitionMap(tx.Sender) == bc.ChainConfig.ShardID || tx.HasBroker) {
		if !tx.Isbrokertx2 {
			value, _ := uint256.FromBig(tx.Value)
			value2, _ := uint256.FromBig(tx.Fee)
			s, _ := hex.DecodeString(tx.Sender)
			// fmt.Println("分片：",strconv.Itoa(int(global.ShardID)),",地址：",tx.Sender,"，更新前的余额：",statedb.GetBalance(common.Address(s)).ToBig().Text(10),",value是：",value.String())
			statedb.SubBalance(common.Address(s), value, tracing.BalanceSubByXBZ)
			statedb.SubBalance(common.Address(s), value2, tracing.BalanceSubByXBZ)
		}
	}

	if bc.Get_PartitionMap(tx.Recipient) == bc.ChainConfig.ShardID || tx.HasBroker {
		if !tx.Isbrokertx1 {

			value, _ := uint256.FromBig(tx.Value)
			s, _ := hex.DecodeString(tx.Recipient)
			fmt.Println("分片：", strconv.Itoa(int(global.ShardID)), "地址：", tx.Recipient, "，更新前的余额：", statedb.GetBalance(common.Address(s)).ToBig().Text(10), ",value是：", value.String())
			statedb.AddBalance(common.Address(s), value, tracing.BalanceAddByXBZ)
			fmt.Println("分片：", strconv.Itoa(int(global.ShardID)), "地址：", tx.Recipient, "，更新后的余额：", statedb.GetBalance(common.Address(s)).ToBig().Text(10))
		}
	}

	//statedb.IntermediateRoot(true)
}

type ChainContext interface {
	GetHeader(common.Hash, uint64) *core.BlockHeader
}

func (bc *BlockChain) GetHeader(hash common.Hash, number uint64) *core.BlockHeader {
	block, err := bc.Storage.GetBlock(hash[:])
	if err != nil {
		return nil
	}
	return block.Header
}
func NewEVMBlockContext(header *core.BlockHeader, chain ChainContext, author *common.Address) vm.BlockContext {
	var (
		beneficiary common.Address
		baseFee     *big.Int
		blobBaseFee *big.Int
		random      *common.Hash
	)
	return vm.BlockContext{
		CanTransfer: CanTransfer,
		Transfer:    Transfer,
		GetHash:     GetHashFn(header, chain),
		Coinbase:    beneficiary,
		BlockNumber: new(big.Int).Set(uint64tobigintptr(header.Number)),
		Time:        uint64(header.Time.UnixMilli()),
		//Difficulty:  new(big.Int).Set(header.Difficulty),
		BaseFee:     baseFee,
		BlobBaseFee: blobBaseFee,
		GasLimit:    header.GasLimit,
		Random:      random,
	}
}
func GetHashFn(ref *core.BlockHeader, chain ChainContext) func(n uint64) common.Hash {
	// Cache will initially contain [refHash.parent],
	// Then fill up with [refHash.p, refHash.pp, refHash.ppp, ...]
	//var cache []common.Hash

	return func(n uint64) common.Hash {
		if ref.Number <= n {
			// This situation can happen if we're doing tracing and using
			// block overrides.
			return common.Hash{}
		}
		// If there's no hash cache yet, make one
		//if len(cache) == 0 {
		//	cache = append(cache, ref.ParentHash)
		//}
		//if idx := ref.Number - n - 1; idx < uint64(len(cache)) {
		//	return cache[idx]
		//}
		// No luck in the cache, but we can start iterating from the last element we already know
		//lastKnownHash := cache[len(cache)-1]
		//lastKnownNumber := ref.Number - uint64(len(cache))
		lastKnownHash := ref.ParentHash
		lastKnownNumber := n

		for {
			header := chain.GetHeader(lastKnownHash, lastKnownNumber)
			if header == nil {
				break
			}
			//cache = append(cache, header.ParentHash)
			lastKnownHash = header.ParentHash
			lastKnownNumber = header.Number - 1
			if n == lastKnownNumber {
				return lastKnownHash
			}
		}
		return common.Hash{}
	}
}

func CanTransfer(db vm.StateDB, addr common.Address, amount *uint256.Int) bool {
	return db.GetBalance(addr).Cmp(amount) >= 0
}

func Transfer(db vm.StateDB, sender, recipient common.Address, amount *uint256.Int) {
	db.SubBalance(sender, amount, tracing.BalanceChangeTransfer)
	db.AddBalance(recipient, amount, tracing.BalanceChangeTransfer)
}
func uint64tobigintptr(u uint64) *big.Int {
	var bigNum big.Int

	bigNum.SetUint64(u)
	return &bigNum
}

// generate (mine) a block, this function return a block
func (bc *BlockChain) GenerateBlock() *core.Block {
	// pack the transactions from the txpool
	txs := bc.Txpool.PackTxs(bc.ChainConfig.BlockSize)
	nodeaccountbyte, _ := hex.DecodeString(global.NodeAccount)
	bh := &core.BlockHeader{
		ParentBlockHash: bc.CurrentBlock.Hash,
		Number:          bc.CurrentBlock.Header.Number + 1,
		Time:            time.Now(),
		GasLimit:        1000000000,
		Coinbase:        common.Address(nodeaccountbyte), //设置coinbase为当前节点的账户地址
	}
	// handle transactions to build root
	//rt := bc.GetUpdateStatusTrie(txs, bh, bc.CurrentBlock)
	bc.GetUpdateStatusTrie(txs, bh, bc.CurrentBlock)

	//global2.Root = rt
	bh.StateRoot = global2.Root.Bytes()
	bh.Root = global2.Root
	bh.TxRoot = GetTxTreeRoot(txs)
	b := core.NewBlock(bh, txs)
	b.Header.Miner = 0
	b.Hash = b.Header.Hash()

	err := bc.Triedb.Commit(b.Header.Root, false)
	if err != nil {
		log.Panic(err)
	}

	global3.Lock.Lock()
	defer global3.Lock.Unlock()
	global3.GlobalStateDB = nil

	return b
}

// new a genisis block, this func will be invoked only once for a blockchain object
func (bc *BlockChain) NewGenisisBlock() *core.Block {
	body := make([]*core.Transaction, 0)
	bh := &core.BlockHeader{
		Number: 0,
	}

	statusTrie := trie.NewEmpty(bc.Triedb)

	bh.StateRoot = statusTrie.Hash().Bytes()
	global2.Root = statusTrie.Hash()
	bh.TxRoot = GetTxTreeRoot(body)
	b := core.NewBlock(bh, body)
	b.Hash = b.Header.Hash()
	return b
}

// add the genisis block in a blockchain
func (bc *BlockChain) AddGenisisBlock(gb *core.Block) {
	bc.Storage.AddBlock(gb)
	newestHash, err := bc.Storage.GetNewestBlockHash()
	if err != nil {
		log.Panic()
	}
	curb, err := bc.Storage.GetBlock(newestHash)
	if err != nil {
		log.Panic()
	}
	bc.CurrentBlock = curb
}

// add a block
func (bc *BlockChain) AddBlock(b *core.Block) {
	_, err := trie.New(trie.TrieID(global2.Root), bc.Triedb)
	if err != nil {
		rt := bc.GetUpdateStatusTrie(b.Body, b.Header, bc.CurrentBlock)
		//global2.Root = rt
		fmt.Println(bc.CurrentBlock.Header.Number+1, "the root = ", rt.Bytes())
	}
	bc.CurrentBlock = b
	bc.Storage.AddBlock(b)
	bc.Storage.DataBase.Sync()
	err = bc.Triedb.Commit(global2.Root, false)
	if err != nil {
		fmt.Println(err)
	}

}

// new a blockchain.
// the ChainConfig is pre-defined to identify the blockchain; the db is the status trie database in disk
func NewBlockChain(cc *params.ChainConfig, db ethdb.Database, Iptable map[uint64]map[uint64]string) (*BlockChain, error) {
	fmt.Println("Generating a new blockchain", db)

	triedb := triedb.NewDatabase(db, &triedb.Config{
		//Cache:     0,
		Preimages: true,
		IsVerkle:  false,
	})
	chainDBfp := "./record/" + fmt.Sprintf("chainDB/S%d_N%d", cc.ShardID, cc.NodeID)
	bc := &BlockChain{
		Db:          db,
		Triedb:      triedb,
		Statedb:     state.NewDatabase(triedb, nil),
		ChainConfig: cc,
		Txpool:      core.NewTxPool(),
		Storage:     storage.NewStorage(chainDBfp, cc),
		//PartitionMap: make(map[string]uint64),
		Iptable: Iptable,
	}
	curHash, err := bc.Storage.GetNewestBlockHash()
	if err != nil {
		fmt.Println("Get newest block hash err")
		// if the Storage bolt database cannot find the newest blockhash,
		// it means the blockchain should be built in height = 0
		if err.Error() == "cannot find the newest block hash" {
			genisisBlock := bc.NewGenisisBlock()
			bc.AddGenisisBlock(genisisBlock)
			fmt.Println("New genisis block")
			return bc, nil
		}
		log.Panic()
	}

	// there is a blockchain in the storage
	fmt.Println("Existing blockchain found")
	curb, err := bc.Storage.GetBlock(curHash)
	if err != nil {
		log.Panic()
	}

	bc.CurrentBlock = curb
	//triedb := trie.NewDatabaseWithConfig(db, &trie.Config{
	//	Cache:     0,
	//	Preimages: true,
	//})
	//bc.triedb = triedb
	// check the existence of the trie database
	_, err = trie.New(trie.TrieID(common.BytesToHash(curb.Header.StateRoot)), triedb)
	if err != nil {
		log.Panic()
	}
	fmt.Println("The status trie can be built")
	fmt.Println("Generated a new blockchain successfully")
	return bc, nil
}

// check a block is valid or not in this blockchain config
func (bc *BlockChain) IsValidBlock(b *core.Block) error {
	return nil
}

// add accounts
func (bc *BlockChain) AddAccounts(ac []string, as []*core.AccountState) {
	fmt.Printf("The len of accounts is %d, now adding the accounts\n", len(ac))

	bh := &core.BlockHeader{
		ParentBlockHash: bc.CurrentBlock.Hash,
		Number:          bc.CurrentBlock.Header.Number + 1,
		Time:            time.Time{},
	}
	// handle transactions to build root
	rt := common.BytesToHash(bc.CurrentBlock.Header.StateRoot)
	if len(ac) != 0 {
		st, err := trie.New(trie.TrieID(common.BytesToHash(bc.CurrentBlock.Header.StateRoot)), bc.Triedb)
		if err != nil {
			log.Panic(err)
		}
		for i, addr := range ac {
			if bc.Get_PartitionMap(addr) == bc.ChainConfig.ShardID {
				ib := new(big.Int)
				ib.Add(ib, as[i].Balance)
				new_state := &core.AccountState{
					Balance: ib,
					Nonce:   as[i].Nonce,
				}
				st.Update([]byte(addr), new_state.Encode())
			}
		}
		rrt, ns := st.Commit(false)
		err = bc.Triedb.Update(common.Hash{}, common.Hash{}, 0, trienode.NewWithNodeSet(ns), nil)
		if err != nil {
			log.Panic(err)
		}
		err = bc.Triedb.Commit(rt, false)
		if err != nil {
			log.Panic(err)
		}
		rt = rrt
	}

	emptyTxs := make([]*core.Transaction, 0)
	bh.StateRoot = rt.Bytes()
	bh.TxRoot = GetTxTreeRoot(emptyTxs)
	b := core.NewBlock(bh, emptyTxs)
	b.Header.Miner = 0
	b.Hash = b.Header.Hash()

	bc.CurrentBlock = b
	bc.Storage.AddBlock(b)
}

// fetch accounts
func (bc *BlockChain) FetchAccounts(addrs []string) []*core.AccountState {
	res := make([]*core.AccountState, 0)
	st, err := trie.New(trie.TrieID(common.BytesToHash(bc.CurrentBlock.Header.StateRoot)), bc.Triedb)
	if err != nil {
		log.Panic(err)
	}
	for _, addr := range addrs {
		asenc, _ := st.Get([]byte(addr))
		var state_a *core.AccountState
		if asenc == nil {
			ib := new(big.Int)
			ib.Add(ib, params.Init_Balance)
			state_a = &core.AccountState{
				Nonce:   uint64(0),
				Balance: ib,
			}
		} else {
			state_a = core.DecodeAS(asenc)
		}
		res = append(res, state_a)
	}
	return res
}
func (bc *BlockChain) FetchAccounts2(addrs []string) ([]*core.AccountState, int, int, []byte, []byte) {

	statedb, _ := state.New2(global2.Root, bc.Statedb)

	res := make([]*core.AccountState, 0)
	//st, err := trie.New(trie.TrieID(common.BytesToHash(bc.CurrentBlock.Header.StateRoot)), bc.Triedb)
	//if err != nil {
	//	log.Panic(err)
	//}
	for _, addr := range addrs {
		//asenc, _ := st.Get([]byte(addr))
		decodeString, _ := hex.DecodeString(addr)
		balance := statedb.GetBalance(common.Address(decodeString))

		var state_a *core.AccountState
		if balance.Cmp(uint256.NewInt(0)) == 0 {
			ib := new(big.Int)
			ib.Add(ib, params.Init_Balance)
			state_a = &core.AccountState{
				Nonce:   uint64(0),
				Balance: ib,
			}
		} else {
			state_a = &core.AccountState{
				Nonce:   uint64(0),
				Balance: balance.ToBig(),
			}
		}
		res = append(res, state_a)
	}

	return res, int(bc.ChainConfig.ShardID), int(bc.CurrentBlock.Header.Number), bc.CurrentBlock.Hash, bc.CurrentBlock.Header.StateRoot
}

// close a blockChain, close the database inferfaces
func (bc *BlockChain) CloseBlockChain() {
	//bc.Storage.DataBase.Close()
	//bc.triedb.CommitPreimages()
	bc.Storage.DataBase.Sync()
	bc.Storage.DataBase.Close()
	bc.Triedb.Commit(bc.CurrentBlock.Header.Root, false)
	bc.Triedb.Close()
}

// print the details of a blockchain
func (bc *BlockChain) PrintBlockChain() string {
	vals := []interface{}{
		bc.CurrentBlock.Header.Number,
		bc.CurrentBlock.Hash,
		bc.CurrentBlock.Header.StateRoot,
		bc.CurrentBlock.Header.Time,
		bc.Triedb,
		// len(bc.Txpool.RelayPool[1]),
	}
	res := fmt.Sprintf("%v\n", vals)
	fmt.Println(res)
	return res
}
