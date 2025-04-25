// Copyright 2016 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package state

import (
	"blockEmulator/global"
	"blockEmulator/networks"
	params2 "blockEmulator/params"
	"blockEmulator/vm/tracing"
	"encoding/json"
	"fmt"
	"maps"
	"math/big"
	"reflect"
	"slices"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
)

type revision struct {
	Id           int
	JournalIndex int
}

// JournalEntry is a modification entry in the state change journal that can be
// reverted on demand.
type JournalEntry interface {
	// revert undoes the changes introduced by this journal entry.
	Revert(*StateDB)

	// dirtied returns the Ethereum address modified by this journal entry.
	dirtied() *common.Address

	// copy returns a deep-copied journal entry.
	copy() JournalEntry

	Execute(*StateDB)
	GetShardID() uint64
}

// journal contains the list of state modifications applied since the last state
// commit. These are tracked to be able to be reverted in the case of an execution
// exception or request for reversal.
type journal struct {
	Entries []JournalEntry         // Current changes tracked by the journal
	dirties map[common.Address]int // Dirty accounts and the number of changes

	ValidRevisions []revision
	nextRevisionId int
}

// newJournal creates a new initialized journal.
func newJournal() *journal {
	return &journal{
		dirties: make(map[common.Address]int),
	}
}

func (j *journal) GetEntry() []JournalEntry{
	return j.Entries
}

// reset clears the journal, after this operation the journal can be used anew.
// It is semantically similar to calling 'newJournal', but the underlying slices
// can be reused.
func (j *journal) reset() {
	j.Entries = j.Entries[:0]
	j.ValidRevisions = j.ValidRevisions[:0]
	clear(j.dirties)
	j.nextRevisionId = 0
}

// snapshot returns an identifier for the current revision of the state.
func (j *journal) snapshot() int {
	id := j.nextRevisionId
	j.nextRevisionId++
	j.ValidRevisions = append(j.ValidRevisions, revision{id, j.length()})
	return id
}

// revertToSnapshot reverts all state changes made since the given revision.
func (j *journal) revertToSnapshot(revid int, s *StateDB) {
	// Find the snapshot in the stack of valid snapshots.
	idx := sort.Search(len(j.ValidRevisions), func(i int) bool {
		return j.ValidRevisions[i].Id >= revid
	})
	if idx == len(j.ValidRevisions) || j.ValidRevisions[idx].Id != revid {
		panic(fmt.Errorf("revision id %v cannot be reverted", revid))
	}
	snapshot := j.ValidRevisions[idx].JournalIndex

	// Replay the journal to undo changes and remove invalidated snapshots
	j.revert(s, snapshot)
	j.ValidRevisions = j.ValidRevisions[:idx]
}

// append inserts a new modification entry to the end of the change journal.
func (j *journal) append(entry JournalEntry) {
	j.Entries = append(j.Entries, entry)
	if addr := entry.dirtied(); addr != nil {
		j.dirties[*addr]++
	}
}
type ExeLogOrRollback struct {
	Log []Wrapper
	Rollback bool
}

func MergeMessage(msgType string, content []byte) []byte {
	b := make([]byte, 30)
	for i, v := range []byte(msgType) {
		b[i] = v
	}
	merge := append(b, content...)
	return merge
}
// revert undoes a batch of journalled modifications along with any reverted
// dirty handling too.
func (j *journal) revert(statedb *StateDB, snapshot int) {
	for i := len(j.Entries) - 1; i >= snapshot; i-- {
		// Undo the changes made by the operation
		if j.Entries[i].GetShardID() != global.ShardID{


			w1:=make([]Wrapper,0)
			w := j.Entries[i]
			switch w.(type) {
			case CreateObjectChange,*CreateObjectChange:
				w1 = append(w1, Wrapper{Type: "CreateObjectChange", OriginalObj: w})
			case CreateContractChange,*CreateContractChange:
				w1 = append(w1, Wrapper{Type: "CreateContractChange", OriginalObj: w})
			case SelfDestructChange,*SelfDestructChange:
				w1 = append(w1, Wrapper{Type: "SelfDestructChange", OriginalObj: w})
			case BalanceChange,*BalanceChange:
				w1 = append(w1, Wrapper{Type: "BalanceChange", OriginalObj: w})
			case NonceChange,*NonceChange:
				w1 = append(w1, Wrapper{Type: "NonceChange", OriginalObj: w})
			case StorageChange,*StorageChange:
				w1 = append(w1, Wrapper{Type: "StorageChange", OriginalObj: w})
			case CodeChange,*CodeChange:
				w1 = append(w1, Wrapper{Type: "CodeChange", OriginalObj: w})
			case RefundChange,*RefundChange:
				w1 = append(w1, Wrapper{Type: "RefundChange", OriginalObj: w})
			case AddLogChange,*AddLogChange:
				w1 = append(w1,Wrapper{Type: "AddLogChange", OriginalObj: w})
			case TouchChange,*TouchChange :
				w1 = append(w1, Wrapper{Type: "TouchChange", OriginalObj: w})
			case AccessListAddAccountChange,*AccessListAddAccountChange:
				w1 = append(w1, Wrapper{Type: "AccessListAddAccountChange", OriginalObj: w})
			case AccessListAddSlotChange,*AccessListAddSlotChange:
				w1 = append(w1, Wrapper{Type: "AccessListAddSlotChange", OriginalObj: w})
			case TransientStorageChange,*TransientStorageChange:
				w1 = append(w1, Wrapper{Type: "TransientStorageChange", OriginalObj: w})
			}
			fmt.Println("跨片回滚操作，type:",w1[0].Type," ",reflect.TypeOf(j.Entries[i]))
			mm:=new(ExeLogOrRollback)
			mm.Rollback = true
			mm.Log = w1
			b,E:=json.Marshal(mm)
			if E != nil {
				fmt.Println(E)
			}
			b1:=MergeMessage("OpExeLogOrRollback",b)
			for i1:=0;i1<len(global.Ip_nodeTable[w.GetShardID()]);i1++{
				go networks.TcpDial(b1,global.Ip_nodeTable[w.GetShardID()][uint64(i1)])
			}


		}else {
			fmt.Println("片内回滚操作，type:",reflect.TypeOf(j.Entries[i]))
			j.Entries[i].Revert(statedb)
		}


		// Drop any dirty tracking induced by the change
		if addr := j.Entries[i].dirtied(); addr != nil {
			if j.dirties[*addr]--; j.dirties[*addr] == 0 {
				delete(j.dirties, *addr)
			}
		}
	}
	j.Entries = j.Entries[:snapshot]
}

// dirty explicitly sets an address to dirty, even if the change entries would
// otherwise suggest it as clean. This method is an ugly hack to handle the RIPEMD
// precompile consensus exception.
func (j *journal) dirty(addr common.Address) {
	j.dirties[addr]++
}

// length returns the current number of entries in the journal.
func (j *journal) length() int {
	return len(j.Entries)
}

// copy returns a deep-copied journal.
func (j *journal) Copy() *journal {
	entries := make([]JournalEntry, 0, j.length())
	for i := 0; i < j.length(); i++ {
		entries = append(entries, j.Entries[i].copy())
	}
	return &journal{
		Entries:        entries,
		dirties:        maps.Clone(j.dirties),
		ValidRevisions: slices.Clone(j.ValidRevisions),
		nextRevisionId: j.nextRevisionId,
	}
}

func (j *journal) logChange(txHash common.Hash) {
	j.append(AddLogChange{txhash: txHash,ShardId: global.ShardID})
}

func (j *journal) createObject(addr common.Address) {
	j.append(CreateObjectChange{account: addr,ShardId: global.ShardID})
}

func (j *journal) createContract(addr common.Address) {
	j.append(CreateContractChange{account: addr,ShardId: global.ShardID})
}

func (j *journal) destruct(addr common.Address) {
	j.append(SelfDestructChange{account: addr,ShardId: global.ShardID})
}

func (j *journal) storageChange(addr common.Address, key, prev, origin ,current common.Hash) {
	j.append(StorageChange{
		account:   addr,
		key:       key,
		prevvalue: prev,
		origvalue: origin,
		current: current,ShardId: global.ShardID,
	})
}

func (j *journal) transientStateChange(addr common.Address, key, prev ,value common.Hash) {
	j.append(TransientStorageChange{
		account:  addr,
		key:      key,
		prevalue: prev,
		value: value,ShardId: global.ShardID,
	})
}

func (j *journal) refundChange(previous uint64,incr uint64) {
	j.append(RefundChange{prev: previous,incr: incr,ShardId: global.ShardID})
}

func (j *journal) balanceChange(addr common.Address, previous uint64,current uint64) {
	j.append(BalanceChange{
		account: addr,
		prev:    previous,
		current: current,
		ShardId: global.ShardID,
	})
}

func (j *journal) setCode(address common.Address,codeHash common.Hash, code []byte) {
	j.append(CodeChange{account: address,codeHash: codeHash,code: code,ShardId: global.ShardID})
}

func (j *journal) nonceChange(address common.Address, prev uint64,current uint64) {
	j.append(NonceChange{
		account: address,
		prev:    prev,
		current: current,ShardId: global.ShardID,
	})
}

func (j *journal) touchChange(address common.Address) {
	j.append(TouchChange{
		account: address,ShardId: global.ShardID,
	})
	if address == ripemd {
		// Explicitly put it in the dirty-cache, which is otherwise generated from
		// flattened journals.
		j.dirty(address)
	}
}

func (j *journal) accessListAddAccount(addr common.Address) {
	j.append(AccessListAddAccountChange{address: addr,ShardId: global.ShardID})
}

func (j *journal) accessListAddSlot(addr common.Address, slot common.Hash) {
	j.append(AccessListAddSlotChange{
		address: addr,
		slot:    slot,
		ShardId: global.ShardID,
	})
}

type (
	// Changes to the account trie.
	CreateObjectChange struct {
		account common.Address
		ShardId uint64
	}
	// CreateContractChange represents an account becoming a contract-account.
	// This event happens prior to executing initcode. The journal-event simply
	// manages the created-flag, in order to allow same-tx destruction.
	CreateContractChange struct {
		account common.Address
		ShardId uint64
	}
	SelfDestructChange struct {
		account common.Address
		ShardId uint64
	}

	// Changes to individual accounts.
	BalanceChange struct {
		account common.Address
		prev    uint64
		current  uint64
		ShardId uint64
	}
	NonceChange struct {
		account common.Address
		prev    uint64
		current uint64
		ShardId uint64
	}
	StorageChange struct {
		account   common.Address
		key       common.Hash
		prevvalue common.Hash
		origvalue common.Hash
		current common.Hash
		ShardId uint64
	}
	CodeChange struct {
		account common.Address
		codeHash common.Hash
		code []byte
		ShardId uint64
	}

	// Changes to other state values.
	RefundChange struct {
		prev uint64
		incr uint64
		ShardId uint64
	}
	AddLogChange struct {
		txhash common.Hash
		ShardId uint64
	}
	TouchChange struct {
		account common.Address
		ShardId uint64
	}

	// Changes to the access list
	AccessListAddAccountChange struct {
		address common.Address
		ShardId uint64
	}
	AccessListAddSlotChange struct {
		address common.Address
		slot    common.Hash
		ShardId uint64
	}

	// Changes to transient storage
	TransientStorageChange struct {
		account       common.Address
		key, prevalue,value common.Hash
		ShardId uint64
	}
)

type Wrapper struct {
	OriginalObj JournalEntry    `json:"-"`    // 存储原始的对象
	Type        string          `json:"type"` //类型
	Data        json.RawMessage `json:"data"` // 存储序列化后的数据
}

func (w *Wrapper) UnmarshalJSON(data []byte) error {
	var aux struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	w.Type = aux.Type
	w.Data = aux.Data

	switch w.Type {
	case "CreateObjectChange":
		w.OriginalObj = &CreateObjectChange{}
	case "CreateContractChange":
		w.OriginalObj = &CreateContractChange{}
	case "SelfDestructChange":
		w.OriginalObj = &SelfDestructChange{}
	case "BalanceChange":
		w.OriginalObj = &BalanceChange{}
	case "NonceChange":
		w.OriginalObj = &NonceChange{}
	case "StorageChange":
		w.OriginalObj = &StorageChange{}
	case "CodeChange":
		w.OriginalObj = &CodeChange{}
	case "RefundChange":
		w.OriginalObj = &RefundChange{}
	case "AddLogChange":
		w.OriginalObj = &AddLogChange{}
	case "TouchChange":
		w.OriginalObj = &TouchChange{}
	case "AccessListAddAccountChange":
		w.OriginalObj = &AccessListAddAccountChange{}
	case "AccessListAddSlotChange":
		w.OriginalObj = &AccessListAddSlotChange{}
	case "TransientStorageChange":
		w.OriginalObj = &TransientStorageChange{}
	default:
		return fmt.Errorf("unknown type: %s", w.Type)
	}
	return json.Unmarshal(w.Data, w.OriginalObj)
}
func (ch CreateObjectChange) GetShardID() uint64{
	return ch.ShardId
}

func (ch CreateContractChange) GetShardID() uint64{
	return ch.ShardId
}

func (ch SelfDestructChange) GetShardID() uint64{
	return ch.ShardId
}

func (ch BalanceChange) GetShardID() uint64{
	return ch.ShardId
}

func (ch NonceChange) GetShardID() uint64{
	return ch.ShardId
}

func (ch StorageChange) GetShardID() uint64{
	return ch.ShardId
}

func (ch CodeChange) GetShardID() uint64{
	return ch.ShardId
}

func (ch RefundChange) GetShardID() uint64{
	return ch.ShardId
}

func (ch AddLogChange) GetShardID() uint64{
	return ch.ShardId
}
func (ch TouchChange) GetShardID() uint64{
	return ch.ShardId
}

func (ch AccessListAddAccountChange) GetShardID() uint64{
	return ch.ShardId
}
func (ch AccessListAddSlotChange) GetShardID() uint64{
	return ch.ShardId
}
func (ch TransientStorageChange) GetShardID() uint64{
	return ch.ShardId
}

func (w *Wrapper) MarshalJSON() ([]byte, error) {
	aux := struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}{
		Type: w.Type,
	}
	var err error
	aux.Data, err = json.Marshal(w.OriginalObj)
	if err != nil {
		return nil, err
	}
	return json.Marshal(aux)
}

func (ch CreateObjectChange) Revert(s *StateDB) {
	delete(s.stateObjects, ch.account)
}

func (ch CreateObjectChange) Execute(s *StateDB) {
	obj := newObject(s, ch.account, nil)
	//添加初始化金额
	ib := new(big.Int)
	ib.Add(ib, params2.Init_Balance)
	obj.setBalance(uint256.MustFromBig(ib))
	//obj.SetState(stringToFixedByteArray("123"), stringToFixedByteArray("123456"))

	//s.journal.createObject(addr)
	s.setStateObject(obj)
}

func (ch CreateObjectChange) dirtied() *common.Address {
	return &ch.account
}

func (ch CreateObjectChange) copy() JournalEntry {
	return CreateObjectChange{
		account: ch.account,
	}
}
func (ch CreateContractChange) Execute(s *StateDB) {
	obj := s.getStateObject(ch.account)
	if !obj.newContract {
		obj.newContract = true
		//s.journal.createContract(ch.account)
	}
}


func (ch CreateContractChange) Revert(s *StateDB) {
	if s.getStateObject(ch.account) ==nil{
		return
	}
	s.getStateObject(ch.account).newContract = false
}

func (ch CreateContractChange) dirtied() *common.Address {
	return nil
}

func (ch CreateContractChange) copy() JournalEntry {
	return CreateContractChange{
		account: ch.account,
	}
}

func (ch SelfDestructChange) Execute(s *StateDB) {
	stateObject := s.getStateObject(ch.account)
	if stateObject == nil {
		return
	}
	// Regardless of whether it is already destructed or not, we do have to
	// journal the balance-change, if we set it to zero here.
	if !stateObject.Balance().IsZero() {
		stateObject.SetBalance(new(uint256.Int), tracing.BalanceDecreaseSelfdestruct)
	}
	// If it is already marked as self-destructed, we do not need to add it
	// for journalling a second time.
	if !stateObject.selfDestructed {
		//s.journal.destruct(addr)
		stateObject.markSelfdestructed()
	}
}

func (ch SelfDestructChange) Revert(s *StateDB) {
	obj := s.getStateObject(ch.account)
	if obj != nil {
		obj.selfDestructed = false
	}
}

func (ch SelfDestructChange) dirtied() *common.Address {
	return &ch.account
}

func (ch SelfDestructChange) copy() JournalEntry {
	return SelfDestructChange{
		account: ch.account,
	}
}

var ripemd = common.HexToAddress("0000000000000000000000000000000000000003")

//TODO
func (ch TouchChange) Execute(s *StateDB) {

}
func (ch TouchChange) Revert(s *StateDB) {
}

func (ch TouchChange) dirtied() *common.Address {
	return &ch.account
}

func (ch TouchChange) copy() JournalEntry {
	return TouchChange{
		account: ch.account,
	}
}
func (ch BalanceChange) Execute(s *StateDB) {
	s.getStateObject(ch.account).setBalance(uint256.NewInt(ch.current))
}
func (ch BalanceChange) Revert(s *StateDB) {
	if s.getStateObject(ch.account) ==nil{
		return
	}
	s.getStateObject(ch.account).setBalance(uint256.NewInt(ch.prev))
}

func (ch BalanceChange) dirtied() *common.Address {
	return &ch.account
}

func (ch BalanceChange) copy() JournalEntry {
	return BalanceChange{
		account: ch.account,
		prev:    ch.prev,
		current: ch.current,
	}
}
func (ch NonceChange) Execute(s *StateDB) {
	s.getStateObject(ch.account).setNonce(ch.current)
}
func (ch NonceChange) Revert(s *StateDB) {
	if s.getStateObject(ch.account) ==nil{
		return
	}
	s.getStateObject(ch.account).setNonce(ch.prev)
}

func (ch NonceChange) dirtied() *common.Address {
	return &ch.account
}

func (ch NonceChange) copy() JournalEntry {
	return NonceChange{
		account: ch.account,
		prev:    ch.prev,
		current: ch.current,
	}
}
func (ch CodeChange) Execute(s *StateDB) {
	s.getStateObject(ch.account).setCode(ch.codeHash, ch.code)
}
func (ch CodeChange) Revert(s *StateDB) {
	if s.getStateObject(ch.account) ==nil{
		return
	}
	s.getStateObject(ch.account).setCode(types.EmptyCodeHash, nil)
}

func (ch CodeChange) dirtied() *common.Address {
	return &ch.account
}

func (ch CodeChange) copy() JournalEntry {
	return CodeChange{
		account: ch.account,
		code:    ch.code,
		codeHash: ch.codeHash,

	}
}
func (ch StorageChange) Execute(s *StateDB) {
	prev, origin := s.getStateObject(ch.account).getState(ch.key)
	if prev == ch.current {
		return
	}
	// New value is different, update and journal the change
	//s.db.journal.storageChange(s.address, key, prev, origin,value)
	s.getStateObject(ch.account).setState(ch.key, ch.current, origin)

}
func (ch StorageChange) Revert(s *StateDB) {
	//TODO
	if s.getStateObject(ch.account) != nil{
		s.getStateObject(ch.account).setState(ch.key, ch.prevvalue, ch.origvalue)
	}

}

func (ch StorageChange) dirtied() *common.Address {
	return &ch.account
}

func (ch StorageChange) copy() JournalEntry {
	return StorageChange{
		account:   ch.account,
		key:       ch.key,
		prevvalue: ch.prevvalue,
		origvalue: ch.origvalue,
		current:   ch.current,
	}
}
func (ch TransientStorageChange) Execute(s *StateDB) {
	prev := s.GetTransientState(ch.account, ch.key)
	if prev == ch.value {
		return
	}
	//s.journal.transientStateChange(addr, key, prev,value)
	s.setTransientState(ch.account, ch.key, ch.value)
}
func (ch TransientStorageChange) Revert(s *StateDB) {

	s.setTransientState(ch.account, ch.key, ch.prevalue)
}

func (ch TransientStorageChange) dirtied() *common.Address {
	return nil
}

func (ch TransientStorageChange) copy() JournalEntry {
	return TransientStorageChange{
		account:  ch.account,
		key:      ch.key,
		prevalue: ch.prevalue,
		value: ch.value,
	}
}
func (ch RefundChange) Execute(s *StateDB) {
	s.refund += ch.incr
}
func (ch RefundChange) Revert(s *StateDB) {
	s.refund = ch.prev
}

func (ch RefundChange) dirtied() *common.Address {
	return nil
}

func (ch RefundChange) copy() JournalEntry {
	return RefundChange{
		prev: ch.prev,
		incr: ch.incr,
	}
}

//TODO
func (ch AddLogChange) Execute(s *StateDB) {

}
func (ch AddLogChange) Revert(s *StateDB) {
	logs := s.logs[ch.txhash]
	if len(logs) == 1 {
		delete(s.logs, ch.txhash)
	} else {
		s.logs[ch.txhash] = logs[:len(logs)-1]
	}
	s.logSize--
}

func (ch AddLogChange) dirtied() *common.Address {
	return nil
}

func (ch AddLogChange) copy() JournalEntry {
	return AddLogChange{
		txhash: ch.txhash,
	}
}

//TODO
func (ch AccessListAddAccountChange) Execute(s *StateDB) {

}
func (ch AccessListAddAccountChange) Revert(s *StateDB) {
	/*
		One important invariant here, is that whenever a (addr, slot) is added, if the
		addr is not already present, the add causes two journal entries:
		- one for the address,
		- one for the (address,slot)
		Therefore, when unrolling the change, we can always blindly delete the
		(addr) at this point, since no storage adds can remain when come upon
		a single (addr) change.
	*/
	//s.accessList.DeleteAddress(ch.address)
}

func (ch AccessListAddAccountChange) dirtied() *common.Address {
	return nil
}

func (ch AccessListAddAccountChange) copy() JournalEntry {
	return AccessListAddAccountChange{
		address: ch.address,
	}
}

//TODO
func (ch AccessListAddSlotChange) Execute(s *StateDB) {

}
func (ch AccessListAddSlotChange) Revert(s *StateDB) {
	//s.accessList.DeleteSlot(ch.address, ch.slot)
}

func (ch AccessListAddSlotChange) dirtied() *common.Address {
	return nil
}

func (ch AccessListAddSlotChange) copy() JournalEntry {
	return AccessListAddSlotChange{
		address: ch.address,
		slot:    ch.slot,
	}
}
