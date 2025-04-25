// The pbft consensus process

package pbft_all

import (
	"blockEmulator/chain"
	"blockEmulator/consensus_shard/pbft_all/pbft_log"
	"blockEmulator/core"
	"blockEmulator/global"
	"blockEmulator/global2"
	"blockEmulator/global3"
	"blockEmulator/message"
	"blockEmulator/networks"
	"blockEmulator/params"
	"blockEmulator/shard"
	"blockEmulator/utils"
	"blockEmulator/vm"
	"blockEmulator/vm/state"
	"blockEmulator/vm/tracing"
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"

	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/ethdb"
)

type PbftConsensusNode struct {
	// the local config about pbft
	RunningNode *shard.Node // the node information
	ShardID     uint64      // denote the ID of the shard (or pbft), only one pbft consensus in a shard
	NodeID      uint64      // denote the ID of the node in the pbft (shard)

	// the data structure for blockchain
	CurChain *chain.BlockChain // all node in the shard maintain the same blockchain
	db       ethdb.Database    // to save the mpt

	// the global config about pbft
	pbftChainConfig *params.ChainConfig          // the chain config in this pbft
	ip_nodeTable    map[uint64]map[uint64]string // denote the ip of the specific node
	node_nums       uint64                       // the number of nodes in this pfbt, denoted by N
	malicious_nums  uint64                       // f, 3f + 1 = N
	view            uint64                       // denote the view of this pbft, the main node can be inferred from this variant

	// the control message and message checking utils in pbft
	sequenceID        uint64                          // the message sequence id of the pbft
	stop              bool                            // send stop signal
	pStop             chan uint64                     // channle for stopping consensus
	requestPool       map[string]*message.Request     // RequestHash to Request
	cntPrepareConfirm map[string]map[*shard.Node]bool // count the prepare confirm message, [messageHash][Node]bool
	cntCommitConfirm  map[string]map[*shard.Node]bool // count the commit confirm message, [messageHash][Node]bool
	isCommitBordcast  map[string]bool                 // denote whether the commit is broadcast
	isReply           map[string]bool                 // denote whether the message is reply
	height2Digest     map[uint64]string               // sequence (block height) -> request, fast read

	// locks about pbft
	sequenceLock sync.Mutex // the lock of sequence
	lock         sync.Mutex // lock the stage
	askForLock   sync.Mutex // lock for asking for a serise of requests
	stopLock     sync.Mutex // lock the stop varient

	// seqID of other Shards, to synchronize
	seqIDMap   map[uint64]uint64
	seqMapLock sync.Mutex

	// logger
	pl *pbft_log.PbftLog
	// tcp control
	tcpln       net.Listener
	tcpPoolLock sync.Mutex

	// to handle the message in the pbft
	ihm ExtraOpInConsensus

	// to handle the message outside of pbft
	ohm OpInterShards

	pbftStage              atomic.Int32 // 1->Preprepare, 2->Prepare, 3->Commit, 4->Done
	pbftLock               sync.Mutex
	conditionalVarpbftLock sync.Cond
}

// generate a pbft consensus for a node
func NewPbftNode(shardID, nodeID uint64, pcc *params.ChainConfig, messageHandleType string) *PbftConsensusNode {
	p := new(PbftConsensusNode)
	p.ip_nodeTable = params.IPmap_nodeTable

	global.Ip_nodeTable = p.ip_nodeTable
	p.node_nums = pcc.Nodes_perShard
	p.ShardID = shardID
	p.NodeID = nodeID
	params.NodeID = nodeID
	params.ShardID = shardID
	global.NodeID = nodeID
	global.ShardID = shardID
	p.pbftChainConfig = pcc
	fp := "./record/ldb/s" + strconv.FormatUint(shardID, 10) + "/n" + strconv.FormatUint(nodeID, 10)
	var err error
	p.db, err = rawdb.NewLevelDBDatabase(fp, 0, 1, "accountState", false)
	if err != nil {
		log.Panic(err)
	}

	p.CurChain, err = chain.NewBlockChain(pcc, p.db, p.ip_nodeTable)
	//global.CurChain = p.CurChain
	if err != nil {
		log.Panic("cannot new a blockchain")
	}

	p.RunningNode = &shard.Node{
		NodeID:  nodeID,
		ShardID: shardID,
		IPaddr:  p.ip_nodeTable[shardID][nodeID],
	}

	p.stop = false
	p.sequenceID = p.CurChain.CurrentBlock.Header.Number + 1
	p.pStop = make(chan uint64)
	p.requestPool = make(map[string]*message.Request)
	p.cntPrepareConfirm = make(map[string]map[*shard.Node]bool)
	p.cntCommitConfirm = make(map[string]map[*shard.Node]bool)
	p.isCommitBordcast = make(map[string]bool)
	p.isReply = make(map[string]bool)
	p.height2Digest = make(map[uint64]string)
	p.malicious_nums = (p.node_nums - 1) / 3
	p.view = 0
	global.View = 0

	p.seqIDMap = make(map[uint64]uint64)

	p.pl = pbft_log.NewPbftLog(shardID, nodeID)
	p.ihm = &RawBrokerPbftExtraHandleMod_forB2E{
		pbftNode: p,
	}
	p.ohm = &RawBrokerOutsideModule_forB2E{
		pbftNode: p,
	}

	// choose how to handle the messages in pbft or beyond pbft
	// switch string(messageHandleType) {
	// case "Broker_b2e":
	// 	p.ihm = &RawBrokerPbftExtraHandleMod_forB2E{
	// 		pbftNode: p,
	// 	}
	// 	p.ohm = &RawBrokerOutsideModule_forB2E{
	// 		pbftNode: p,
	// 	}
	// case "CLPA_Broker":
	// 	ncdm := dataSupport.NewCLPADataSupport()
	// 	p.ihm = &CLPAPbftInsideExtraHandleMod_forBroker{
	// 		pbftNode: p,
	// 		cdm:      ncdm,
	// 	}
	// 	p.ohm = &CLPABrokerOutsideModule{
	// 		pbftNode: p,
	// 		cdm:      ncdm,
	// 	}
	// case "CLPA":
	// 	ncdm := dataSupport.NewCLPADataSupport()
	// 	p.ihm = &CLPAPbftInsideExtraHandleMod{
	// 		pbftNode: p,
	// 		cdm:      ncdm,
	// 	}
	// 	p.ohm = &CLPARelayOutsideModule{
	// 		pbftNode: p,
	// 		cdm:      ncdm,
	// 	}
	// case "Broker":
	// 	p.ihm = &RawBrokerPbftExtraHandleMod{
	// 		pbftNode: p,
	// 	}
	// 	p.ohm = &RawBrokerOutsideModule{
	// 		pbftNode: p,
	// 	}
	// default:
	// 	p.ihm = &RawRelayPbftExtraHandleMod{
	// 		pbftNode: p,
	// 	}
	// 	p.ohm = &RawRelayOutsideModule{
	// 		pbftNode: p,
	// 	}
	// }
	p.conditionalVarpbftLock = *sync.NewCond(&p.pbftLock)
	p.pbftStage.Store(1)
	return p
}

// handle the raw message, send it to corresponded interfaces
func (p *PbftConsensusNode) handleMessage(msg []byte) {
	msgType, content := message.SplitMessage(msg)
	switch msgType {
	// pbft inside message type
	case message.CPrePrepare:
		go p.handlePrePrepare(content)
	case message.CPrepare:
		go p.handlePrepare(content)
	case message.CCommit:
		go p.handleCommit(content)
	case message.CRequestOldrequest:
		p.handleRequestOldSeq(content)
	case message.CSendOldrequest:
		p.handleSendOldSeq(content)
	case message.CStop:
		p.WaitToStop()
	case message.CClientRequestAccountState:
		p.handleRequestAccountState(content)
	case message.CClientRequestTransaction:
		p.handleRequestTransaction(content)
	case message.CResponseNodeAccount:
		go p.handleResponseNodeAcc(content)
	case message.CQueryNodeAccount:
		go p.handleQueryNodeAcc(content)
	case message.CQueryAccount:
		go p.handleQueryAcc(content)
	case message.CResponseAccount:
		go p.handleResponseAcc(content)
	case message.COpbalanceReq:
		go p.handleOpbanlanceReq(content)
	case message.COpbalanceRes:
		go p.handleOpbanlanceRes(content)
	case message.COpDelegateCallReq:
		go p.handleOpDelegateCallReq(content)
	case message.COpDelegateCallRes:
		go p.handleOpDelegateCallRes(content)
	case message.COpCallCodeReq:
		go p.handleOpCallCodeReq(content)
	case message.COpCallCodeRes:
		go p.handleOpCallCodeRes(content)
	case message.COpCallReq:
		go p.handleOpCallReq(content)
	case message.COpCallRes:
		go p.handleOpCallRes(content)
	case message.COpStaticCallReq:
		go p.handleOpStaticCallReq(content)
	case message.COpStaticCallRes:
		go p.handleOpStaticCallRes(content)
	case message.COpExeLogOrRollback:
		go p.handleCOpExeLogOrRollback(content)
	case message.COpCreateReq:
		go p.handleOpCreateReq(content)
	case message.COpCreateRes:
		go p.handleOpCreateRes(content)
	case message.CQueryContractResultReq:
		go p.handleQueryContractResultReq(content)
	case message.CQueryContractResultRes:
		go p.handleQueryContractResultRes(content)

	// handle the message from outside
	default:
		go p.ohm.HandleMessageOutsidePBFT(msgType, content)
	}
}

func (p *PbftConsensusNode) handleQueryContractResultReq(content []byte) {
	m := new(message.QueryContractResultReq)
	json.Unmarshal(content, m)
	var res []byte
	var err error
	now := time.Now()
	for time.Since(now).Milliseconds() < 10000 {
		res, err = p.CurChain.Storage.GetContractRes(m.UUID)
		if err == nil {
			break
		}
	}

	if err != nil {
		m1 := new(message.QueryContractResultRes)
		m1.Success = false
		m1.UUID = m.UUID
		b, _ := json.Marshal(m1)
		b1 := message.MergeMessage(message.CQueryContractResultRes, b)
		go networks.TcpDial(b1, global.Ip_nodeTable[m.FromShardID][m.FromNodeID])
		return
	}

	m1 := new(message.QueryContractResultRes)
	m1.Success = true
	m1.Result = res
	m1.UUID = m.UUID
	b, _ := json.Marshal(m1)
	b1 := message.MergeMessage(message.CQueryContractResultRes, b)
	go networks.TcpDial(b1, global.Ip_nodeTable[m.FromShardID][m.FromNodeID])

}
func (p *PbftConsensusNode) handleQueryContractResultRes(content []byte) {
	m := new(message.QueryContractResultRes)
	json.Unmarshal(content, m)
	if m.Success {
		s := hex.EncodeToString(m.Result)
		fmt.Println("查询合约执行结果成功，uuid为" + m.UUID + ",结果为：" + s)
	} else {
		fmt.Println("查询失败：" + m.UUID)
	}
}

func (p *PbftConsensusNode) handleOpCreateReq(content []byte) {
	m := new(message.OpCreateReq)
	err := json.Unmarshal(content, m)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println()
	fmt.Println("创建的地址是", hex.EncodeToString(m.Addr[:]))
	fmt.Println()
	global.ExeTxLock.LockWithOwner(m.UUID)
	defer global.ExeTxLock.Unlock()
	blockContext := chain.NewEVMBlockContext(p.CurChain.CurrentBlock.Header, p.CurChain, nil)
	statedb, _, sn := state.New(global2.Root, p.CurChain.Statedb)

	contractHash := statedb.GetCodeHash(m.Addr)
	storageRoot := statedb.GetStorageRoot(m.Addr)
	res := new(message.OpCreateRes)
	if statedb.GetNonce(m.Addr) != 0 ||
		(contractHash != (common.Hash{}) && contractHash != types.EmptyCodeHash) || // non-empty code
		(storageRoot != (common.Hash{}) && storageRoot != types.EmptyRootHash) { // non-empty storage
		//if evm.Config.Tracer != nil && evm.Config.Tracer.OnGasChange != nil {
		//	evm.Config.Tracer.OnGasChange(gas, 0, tracing.GasChangeCallFailedExecution)
		//}
		//return nil, common.Address{}, 0, errors.New("contract address collision")

		fmt.Println("创建合约时地址冲突。。。 m.Addr= ", m.Addr, " nonce为", statedb.GetNonce(m.Addr), " contractHash=", hex.EncodeToString(contractHash[:]), " storageRoot=", hex.EncodeToString(storageRoot[:]))
		res.Ret = nil
		res.Addr = common.Address{}
		res.Id = m.Id
		res.Err = "contract address collision"
		b, E := json.Marshal(res)
		if E != nil {
			fmt.Println(E)
		}
		b1 := message.MergeMessage(message.COpCreateRes, b)
		go networks.TcpDial(b1, global.Ip_nodeTable[m.FromShardId][m.FromNodeId])
		return
	}

	snapshot := statedb.Snapshot()
	if !statedb.Exist(m.Addr) {
		statedb.CreateAccount(m.Addr)
	}
	ib := new(big.Int)
	ib.Add(ib, params.Init_Balance)
	//statedb.SubBalance(m.Addr, uint256.MustFromBig(ib), tracing.BalanceInilizeByXBZ)
	// CreateContract means that regardless of whether the account previously existed
	// in the state trie or not, it _now_ becomes created as a _contract_ account.
	// This is performed _prior_ to executing the initcode,  since the initcode
	// acts inside that account.
	statedb.CreateContract(m.Addr)
	statedb.AddBalance(m.Addr, uint256.NewInt(m.Value), tracing.BalanceChangeTransfer)

	vmenv := vm.NewEVM(blockContext, vm.TxContext{}, statedb, nil, vm.Config{})

	//statedb.SetTxContext(common.Hash(tx.TxHash), idx)
	addrcopy := common.Address(m.Addr)
	contract := vm.NewContract(vm.AccountRef(m.Caller), vm.AccountRef(addrcopy), uint256.NewInt(m.Value), m.Gas)
	contract.Gas = 100000000
	contract.UUID = m.UUID
	contract.Code = m.Code
	contract.CodeAddr = &m.Addr
	contract.IsDeployment = true

	ret, err := vmenv.InitNewContract(contract, m.Addr, uint256.NewInt(m.Value))

	if err != nil {
		fmt.Println("handleOpCreateReq错误, ", err.Error())
		statedb.RevertToSnapshot(snapshot)
		res.Err = err.Error()
	} else {
		res.Err = ""
		j := statedb.GetJournal().Copy()
		idx_ := sort.Search(len(j.ValidRevisions), func(i int) bool {
			return j.ValidRevisions[i].Id >= snapshot
		})
		s1 := j.ValidRevisions[idx_].JournalIndex
		var w1 []state.Wrapper
		for i := s1; i <= len(j.Entries)-1; i++ {
			w := j.Entries[i]
			switch w.(type) {
			case state.CreateObjectChange, *state.CreateObjectChange:
				w1 = append(w1, state.Wrapper{Type: "CreateObjectChange", OriginalObj: w})
			case state.CreateContractChange, *state.CreateContractChange:
				w1 = append(w1, state.Wrapper{Type: "CreateContractChange", OriginalObj: w})
			case state.SelfDestructChange, *state.SelfDestructChange:
				w1 = append(w1, state.Wrapper{Type: "SelfDestructChange", OriginalObj: w})
			case state.BalanceChange, *state.BalanceChange:
				w1 = append(w1, state.Wrapper{Type: "BalanceChange", OriginalObj: w})
			case state.NonceChange, *state.NonceChange:
				w1 = append(w1, state.Wrapper{Type: "NonceChange", OriginalObj: w})
			case state.StorageChange, *state.StorageChange:
				w1 = append(w1, state.Wrapper{Type: "StorageChange", OriginalObj: w})
			case state.CodeChange, *state.CodeChange:
				w1 = append(w1, state.Wrapper{Type: "CodeChange", OriginalObj: w})
			case state.RefundChange, *state.RefundChange:
				w1 = append(w1, state.Wrapper{Type: "RefundChange", OriginalObj: w})
			case state.AddLogChange, *state.AddLogChange:
				w1 = append(w1, state.Wrapper{Type: "AddLogChange", OriginalObj: w})
			case state.TouchChange, *state.TouchChange:
				w1 = append(w1, state.Wrapper{Type: "TouchChange", OriginalObj: w})
			case state.AccessListAddAccountChange, *state.AccessListAddAccountChange:
				w1 = append(w1, state.Wrapper{Type: "AccessListAddAccountChange", OriginalObj: w})
			case state.AccessListAddSlotChange, *state.AccessListAddSlotChange:
				w1 = append(w1, state.Wrapper{Type: "AccessListAddSlotChange", OriginalObj: w})
			case state.TransientStorageChange, *state.TransientStorageChange:
				w1 = append(w1, state.Wrapper{Type: "TransientStorageChange", OriginalObj: w})
			default:
				fmt.Println("exetx error unknown type")
			}

		}

		mm := new(state.ExeLogOrRollback)
		mm.Rollback = false
		mm.Log = w1

		bb, E := json.Marshal(mm)
		if E != nil {
			fmt.Println(E)
		}
		bb1 := message.MergeMessage(message.COpExeLogOrRollback, bb)
		for i := 1; i < len(global.Ip_nodeTable[global.ShardID]); i++ {
			go networks.TcpDial(bb1, global.Ip_nodeTable[global.ShardID][uint64(i)])
		}
	}

	res.Ret = ret
	res.Id = m.Id

	if err == nil {
		var w []state.Wrapper
		entry := statedb.GetJournal().GetEntry()
		for _, item := range entry {
			switch v := item.(type) {
			case state.CreateObjectChange, *state.CreateObjectChange:
				w = append(w, state.Wrapper{Type: "CreateObjectChange", OriginalObj: item})
			case state.CreateContractChange, *state.CreateContractChange:
				w = append(w, state.Wrapper{Type: "CreateContractChange", OriginalObj: item})
			case state.SelfDestructChange, *state.SelfDestructChange:
				w = append(w, state.Wrapper{Type: "SelfDestructChange", OriginalObj: item})
			case state.BalanceChange, *state.BalanceChange:
				w = append(w, state.Wrapper{Type: "BalanceChange", OriginalObj: item})
			case state.NonceChange, *state.NonceChange:
				w = append(w, state.Wrapper{Type: "NonceChange", OriginalObj: item})
			case state.StorageChange, *state.StorageChange:
				w = append(w, state.Wrapper{Type: "StorageChange", OriginalObj: item})
			case state.CodeChange, *state.CodeChange:
				w = append(w, state.Wrapper{Type: "CodeChange", OriginalObj: item})
			case state.RefundChange, *state.RefundChange:
				w = append(w, state.Wrapper{Type: "RefundChange", OriginalObj: item})
			case state.AddLogChange, *state.AddLogChange:
				w = append(w, state.Wrapper{Type: "AddLogChange", OriginalObj: item})
			case state.TouchChange, *state.TouchChange:
				w = append(w, state.Wrapper{Type: "TouchChange", OriginalObj: item})
			case state.AccessListAddAccountChange, *state.AccessListAddAccountChange:
				w = append(w, state.Wrapper{Type: "AccessListAddAccountChange", OriginalObj: item})
			case state.AccessListAddSlotChange, *state.AccessListAddSlotChange:
				w = append(w, state.Wrapper{Type: "AccessListAddSlotChange", OriginalObj: item})
			case state.TransientStorageChange, *state.TransientStorageChange:
				w = append(w, state.Wrapper{Type: "TransientStorageChange", OriginalObj: item})
			default:
				fmt.Println("error: type ", v)
			}
		}
		res.Journal = append(res.Journal, w...)
	}

	if err != nil {
		res.Journal = nil
	} else {
		var E1 error
		//statedb.IntermediateRoot(false)
		if sn == "new" {
			global2.Root, E1 = statedb.Commit(0, false)
			if E1 != nil {
				fmt.Println(E1)
			}
			err1 := p.CurChain.Triedb.Commit(global2.Root, false)
			if err1 != nil {
				fmt.Println(err1)
			}
			global3.Lock.Lock()
			global3.GlobalStateDB = nil
			global3.Lock.Unlock()
		}

		//statedb2, _ := state.New2(global2.Root, p.CurChain.Statedb)
		//var aa = statedb2.GetCode(m.Addr) ==nil
		//fmt.Println("读取的code是否为空：",aa)
	}

	b, E := json.Marshal(res)
	if E != nil {
		fmt.Println(E)
	}
	b1 := message.MergeMessage(message.COpCreateRes, b)
	go networks.TcpDial(b1, global.Ip_nodeTable[m.FromShardId][m.FromNodeId])

}

func (p *PbftConsensusNode) handleOpCreateRes(content []byte) {
	m := new(message.OpCreateRes)
	err := json.Unmarshal(content, m)
	if err != nil {
		fmt.Println(err)
	}
	m1 := new(state.OpCreateRes)
	m1.Ret = m.Ret
	m1.Id = m.Id
	m1.Addr = m.Addr
	m1.Err = m.Err
	m1.Journal = m.Journal
	state.OpCreateResMapLock.Lock()
	state.OpCreateResMap[m.Id] = *m1
	state.OpCreateResMapLock.Unlock()

}

func (p *PbftConsensusNode) handleCOpExeLogOrRollback(content []byte) {

	if p.NodeID != 0 {
		return
	}

	m := new(state.ExeLogOrRollback)
	err := json.Unmarshal(content, m)
	if err != nil {
		fmt.Println(err)
	}
	statedb, _, sn := state.New(global2.Root, p.CurChain.Statedb)
	if m.Rollback {
		for i := len(m.Log) - 1; i >= 0; i-- {
			m.Log[i].OriginalObj.Revert(statedb)
		}
	} else {
		for _, item := range m.Log {
			item.OriginalObj.Execute(statedb)
		}
	}
	if sn == "new" {
		global2.Root, err = statedb.Commit(0, false)
		err1 := p.CurChain.Triedb.Commit(global2.Root, false)
		if err1 != nil {
			fmt.Println(err1)
		}
		global3.Lock.Lock()
		global3.GlobalStateDB = nil
		global3.Lock.Unlock()
	}
}

func (p *PbftConsensusNode) handleOpStaticCallReq(content []byte) {
	m := new(message.OpStaticCallReq)
	err := json.Unmarshal(content, m)
	if err != nil {
		fmt.Println(err)
	}

	global.ExeTxLock.LockWithOwner(m.UUID)
	defer global.ExeTxLock.Unlock()

	blockContext := chain.NewEVMBlockContext(p.CurChain.CurrentBlock.Header, p.CurChain, nil)
	statedb, _, sn := state.New(global2.Root, p.CurChain.Statedb)
	snapshot := statedb.Snapshot()
	//decodeString, _ := hex.DecodeString(msg.Address)
	code := statedb.GetCode(m.Addr)
	//blockContext.Coinbase = blockHeader.Coinbase
	//txContext := NewEVMTxContext(msg)
	//fmt.Println("NewEVM")
	vmenv := vm.NewEVM(blockContext, vm.TxContext{}, statedb, nil, vm.Config{})
	//statedb.SetTxContext(common.Hash(tx.TxHash), idx)
	addrcopy := common.Address(m.Addr)

	contract := vm.NewContract(vm.AccountRef(m.Caller), vm.AccountRef(addrcopy), uint256.NewInt(0), m.Gas)
	contract.UUID = m.UUID
	contract.Gas = 100000000
	contract.SetCallCode(&addrcopy, statedb.GetCodeHash(addrcopy), code)

	ret, err := vmenv.Interpreter().Run(contract, m.Input, true)

	r := new(message.OpStaticCallRes)

	if err != nil {
		fmt.Println("handleOpStaticCallReq错误，", err.Error())
		statedb.RevertToSnapshot(snapshot)
		r.Err = err.Error()
	} else {
		//j := statedb.GetJournal().Copy()
		//idx_ := sort.Search(len(j.ValidRevisions), func(i int) bool {
		//	return j.ValidRevisions[i].Id >= snapshot
		//})
		//s1 := j.ValidRevisions[idx_].JournalIndex
		//var w1 []state.Wrapper
		//for i := s1; i <= len(j.Entries)-1; i++ {
		//	w := j.Entries[i]
		//	switch w.(type) {
		//	case state.CreateObjectChange, *state.CreateObjectChange:
		//		w1 = append(w1, state.Wrapper{Type: "CreateObjectChange", OriginalObj: w})
		//	case state.CreateContractChange, *state.CreateContractChange:
		//		w1 = append(w1, state.Wrapper{Type: "CreateContractChange", OriginalObj: w})
		//	case state.SelfDestructChange, *state.SelfDestructChange:
		//		w1 = append(w1, state.Wrapper{Type: "SelfDestructChange", OriginalObj: w})
		//	case state.BalanceChange, *state.BalanceChange:
		//		w1 = append(w1, state.Wrapper{Type: "BalanceChange", OriginalObj: w})
		//	case state.NonceChange, *state.NonceChange:
		//		w1 = append(w1, state.Wrapper{Type: "NonceChange", OriginalObj: w})
		//	case state.StorageChange, *state.StorageChange:
		//		w1 = append(w1, state.Wrapper{Type: "StorageChange", OriginalObj: w})
		//	case state.CodeChange, *state.CodeChange:
		//		w1 = append(w1, state.Wrapper{Type: "CodeChange", OriginalObj: w})
		//	case state.RefundChange, *state.RefundChange:
		//		w1 = append(w1, state.Wrapper{Type: "RefundChange", OriginalObj: w})
		//	case state.AddLogChange, *state.AddLogChange:
		//		w1 = append(w1, state.Wrapper{Type: "AddLogChange", OriginalObj: w})
		//	case state.TouchChange, *state.TouchChange:
		//		w1 = append(w1, state.Wrapper{Type: "TouchChange", OriginalObj: w})
		//	case state.AccessListAddAccountChange, *state.AccessListAddAccountChange:
		//		w1 = append(w1, state.Wrapper{Type: "AccessListAddAccountChange", OriginalObj: w})
		//	case state.AccessListAddSlotChange, *state.AccessListAddSlotChange:
		//		w1 = append(w1, state.Wrapper{Type: "AccessListAddSlotChange", OriginalObj: w})
		//	case state.TransientStorageChange, *state.TransientStorageChange:
		//		w1 = append(w1, state.Wrapper{Type: "TransientStorageChange", OriginalObj: w})
		//	default:
		//		fmt.Println("exetx error unknown type")
		//	}
		//
		//}
		//
		//mm := new(state.ExeLogOrRollback)
		//mm.Rollback = false
		//mm.Log = w1
		//
		//bb, E := json.Marshal(mm)
		//if E != nil {
		//	fmt.Println(E)
		//}
		//bb1 := message.MergeMessage(message.COpExeLogOrRollback, bb)
		//for i := 1; i < len(global.Ip_nodeTable[global.ShardID]); i++ {
		//	go networks.TcpDial(bb1, global.Ip_nodeTable[global.ShardID][uint64(i)])
		//}
		r.Err = ""

	}

	r.Id = m.Id
	r.UUID = m.UUID
	r.Ret = ret
	//var w []state.Wrapper
	//entry := statedb.GetJournal().GetEntry()
	//for _, item := range entry {
	//	switch v := item.(type) {
	//	case state.CreateObjectChange, *state.CreateObjectChange:
	//		w = append(w, state.Wrapper{Type: "CreateObjectChange", OriginalObj: item})
	//	case state.CreateContractChange, *state.CreateContractChange:
	//		w = append(w, state.Wrapper{Type: "CreateContractChange", OriginalObj: item})
	//	case state.SelfDestructChange, *state.SelfDestructChange:
	//		w = append(w, state.Wrapper{Type: "SelfDestructChange", OriginalObj: item})
	//	case state.BalanceChange, *state.BalanceChange:
	//		w = append(w, state.Wrapper{Type: "BalanceChange", OriginalObj: item})
	//	case state.NonceChange, *state.NonceChange:
	//		w = append(w, state.Wrapper{Type: "NonceChange", OriginalObj: item})
	//	case state.StorageChange, *state.StorageChange:
	//		w = append(w, state.Wrapper{Type: "StorageChange", OriginalObj: item})
	//	case state.CodeChange, *state.CodeChange:
	//		w = append(w, state.Wrapper{Type: "CodeChange", OriginalObj: item})
	//	case state.RefundChange, *state.RefundChange:
	//		w = append(w, state.Wrapper{Type: "RefundChange", OriginalObj: item})
	//	case state.AddLogChange, *state.AddLogChange:
	//		w = append(w, state.Wrapper{Type: "AddLogChange", OriginalObj: item})
	//	case state.TouchChange, *state.TouchChange:
	//		w = append(w, state.Wrapper{Type: "TouchChange", OriginalObj: item})
	//	case state.AccessListAddAccountChange, *state.AccessListAddAccountChange:
	//		w = append(w, state.Wrapper{Type: "AccessListAddAccountChange", OriginalObj: item})
	//	case state.AccessListAddSlotChange, *state.AccessListAddSlotChange:
	//		w = append(w, state.Wrapper{Type: "AccessListAddSlotChange", OriginalObj: item})
	//	case state.TransientStorageChange, *state.TransientStorageChange:
	//		w = append(w, state.Wrapper{Type: "TransientStorageChange", OriginalObj: item})
	//	default:
	//		fmt.Println("error: type ", v)
	//	}
	//}
	//r.Journal = append(r.Journal, w...)

	if err != nil {
		//r.Journal = nil
	} else {

	}
	if sn == "new" {
		global2.Root, _ = statedb.Commit(0, false)
		err1 := p.CurChain.Triedb.Commit(global2.Root, false)
		if err1 != nil {
			fmt.Println(err1)
		}
		global3.Lock.Lock()
		global3.GlobalStateDB = nil
		global3.Lock.Unlock()
	}
	b, e := json.Marshal(r)
	if e != nil {
		fmt.Println(e)
	}
	b1 := message.MergeMessage(message.COpStaticCallRes, b)

	fmt.Println("handleopstaticcallreq返回的结果是", hex.EncodeToString(r.Ret))
	fmt.Println("uuid是", r.UUID)

	go networks.TcpDial(b1, global.Ip_nodeTable[m.FromShardId][m.FromNodeId])

}

func (p *PbftConsensusNode) handleOpCallReq(content []byte) {
	m := new(message.OpCallReq)
	err := json.Unmarshal(content, m)
	if err != nil {
		fmt.Println(err)
	}

	global.ExeTxLock.LockWithOwner(m.UUID)
	defer global.ExeTxLock.Unlock()

	Str := hex.EncodeToString(m.Addr[:])
	for true {
		global.GLobalLock.Lock()
		if _, ok1 := global.GlobalLockMap[Str]; ok1 {
			global.GLobalLock.Unlock()
			continue
		}
		global.GlobalLockMap[Str] = true
		global.GLobalLock.Unlock()
		break
	}
	defer func() {
		global.GLobalLock.Lock()
		delete(global.GlobalLockMap, Str)
		global.GLobalLock.Unlock()
	}()

	fmt.Println("开始handleOpCallReq,value为", m.Value)
	blockContext := chain.NewEVMBlockContext(p.CurChain.CurrentBlock.Header, p.CurChain, nil)
	statedb, _, sn := state.New(global2.Root, p.CurChain.Statedb)
	snapshot := statedb.Snapshot()
	//decodeString, _ := hex.DecodeString(msg.Address)
	code := statedb.GetCode(m.Addr)
	//blockContext.Coinbase = blockHeader.Coinbase
	//txContext := NewEVMTxContext(msg)
	//fmt.Println("NewEVM")
	vmenv := vm.NewEVM(blockContext, vm.TxContext{}, statedb, nil, vm.Config{})
	//statedb.SetTxContext(common.Hash(tx.TxHash), idx)
	addrcopy := common.Address(m.Addr)
	statedb.AddBalance(m.Addr, uint256.NewInt(m.Value), tracing.BalanceChangeTransfer)
	contract := vm.NewContract(vm.AccountRef(m.Caller), vm.AccountRef(addrcopy), uint256.NewInt(m.Value), m.Gas)
	contract.UUID = m.UUID
	contract.Gas = 100000000
	contract.SetCallCode(&addrcopy, statedb.GetCodeHash(addrcopy), code)

	ret, err := vmenv.Interpreter().Run(contract, m.Input, false)

	r := new(message.OpCallRes)

	if err != nil {
		fmt.Println("handleOpCallReq错误，", err.Error())
		statedb.RevertToSnapshot(snapshot)
		r.Err = err.Error()
	} else {
		j := statedb.GetJournal().Copy()
		idx_ := sort.Search(len(j.ValidRevisions), func(i int) bool {
			return j.ValidRevisions[i].Id >= snapshot
		})
		s1 := j.ValidRevisions[idx_].JournalIndex
		var w1 []state.Wrapper
		for i := s1; i <= len(j.Entries)-1; i++ {
			w := j.Entries[i]
			switch w.(type) {
			case state.CreateObjectChange, *state.CreateObjectChange:
				w1 = append(w1, state.Wrapper{Type: "CreateObjectChange", OriginalObj: w})
			case state.CreateContractChange, *state.CreateContractChange:
				w1 = append(w1, state.Wrapper{Type: "CreateContractChange", OriginalObj: w})
			case state.SelfDestructChange, *state.SelfDestructChange:
				w1 = append(w1, state.Wrapper{Type: "SelfDestructChange", OriginalObj: w})
			case state.BalanceChange, *state.BalanceChange:
				w1 = append(w1, state.Wrapper{Type: "BalanceChange", OriginalObj: w})
			case state.NonceChange, *state.NonceChange:
				w1 = append(w1, state.Wrapper{Type: "NonceChange", OriginalObj: w})
			case state.StorageChange, *state.StorageChange:
				w1 = append(w1, state.Wrapper{Type: "StorageChange", OriginalObj: w})
			case state.CodeChange, *state.CodeChange:
				w1 = append(w1, state.Wrapper{Type: "CodeChange", OriginalObj: w})
			case state.RefundChange, *state.RefundChange:
				w1 = append(w1, state.Wrapper{Type: "RefundChange", OriginalObj: w})
			case state.AddLogChange, *state.AddLogChange:
				w1 = append(w1, state.Wrapper{Type: "AddLogChange", OriginalObj: w})
			case state.TouchChange, *state.TouchChange:
				w1 = append(w1, state.Wrapper{Type: "TouchChange", OriginalObj: w})
			case state.AccessListAddAccountChange, *state.AccessListAddAccountChange:
				w1 = append(w1, state.Wrapper{Type: "AccessListAddAccountChange", OriginalObj: w})
			case state.AccessListAddSlotChange, *state.AccessListAddSlotChange:
				w1 = append(w1, state.Wrapper{Type: "AccessListAddSlotChange", OriginalObj: w})
			case state.TransientStorageChange, *state.TransientStorageChange:
				w1 = append(w1, state.Wrapper{Type: "TransientStorageChange", OriginalObj: w})
			default:
				fmt.Println("exetx error unknown type")
			}

		}

		mm := new(state.ExeLogOrRollback)
		mm.Rollback = false
		mm.Log = w1

		bb, E := json.Marshal(mm)
		if E != nil {
			fmt.Println(E)
		}
		bb1 := message.MergeMessage(message.COpExeLogOrRollback, bb)
		for i := 1; i < len(global.Ip_nodeTable[global.ShardID]); i++ {
			go networks.TcpDial(bb1, global.Ip_nodeTable[global.ShardID][uint64(i)])
		}
		r.Err = ""

	}

	r.Id = m.Id
	r.UUID = m.UUID
	r.Ret = ret
	var w []state.Wrapper
	entry := statedb.GetJournal().GetEntry()
	for _, item := range entry {
		switch v := item.(type) {
		case state.CreateObjectChange, *state.CreateObjectChange:
			w = append(w, state.Wrapper{Type: "CreateObjectChange", OriginalObj: item})
		case state.CreateContractChange, *state.CreateContractChange:
			w = append(w, state.Wrapper{Type: "CreateContractChange", OriginalObj: item})
		case state.SelfDestructChange, *state.SelfDestructChange:
			w = append(w, state.Wrapper{Type: "SelfDestructChange", OriginalObj: item})
		case state.BalanceChange, *state.BalanceChange:
			w = append(w, state.Wrapper{Type: "BalanceChange", OriginalObj: item})
		case state.NonceChange, *state.NonceChange:
			w = append(w, state.Wrapper{Type: "NonceChange", OriginalObj: item})
		case state.StorageChange, *state.StorageChange:
			w = append(w, state.Wrapper{Type: "StorageChange", OriginalObj: item})
		case state.CodeChange, *state.CodeChange:
			w = append(w, state.Wrapper{Type: "CodeChange", OriginalObj: item})
		case state.RefundChange, *state.RefundChange:
			w = append(w, state.Wrapper{Type: "RefundChange", OriginalObj: item})
		case state.AddLogChange, *state.AddLogChange:
			w = append(w, state.Wrapper{Type: "AddLogChange", OriginalObj: item})
		case state.TouchChange, *state.TouchChange:
			w = append(w, state.Wrapper{Type: "TouchChange", OriginalObj: item})
		case state.AccessListAddAccountChange, *state.AccessListAddAccountChange:
			w = append(w, state.Wrapper{Type: "AccessListAddAccountChange", OriginalObj: item})
		case state.AccessListAddSlotChange, *state.AccessListAddSlotChange:
			w = append(w, state.Wrapper{Type: "AccessListAddSlotChange", OriginalObj: item})
		case state.TransientStorageChange, *state.TransientStorageChange:
			w = append(w, state.Wrapper{Type: "TransientStorageChange", OriginalObj: item})
		default:
			fmt.Println("error: type ", v)
		}
	}
	r.Journal = append(r.Journal, w...)

	if err != nil {
		r.Journal = nil
	} else {

	}
	if sn == "new" {
		global2.Root, _ = statedb.Commit(0, false)
		err1 := p.CurChain.Triedb.Commit(global2.Root, false)
		if err1 != nil {
			fmt.Println(err1)
		}
		global3.Lock.Lock()
		global3.GlobalStateDB = nil
		global3.Lock.Unlock()
	}
	b, e := json.Marshal(r)
	if e != nil {
		fmt.Println(e)
	}
	b1 := message.MergeMessage(message.COpCallRes, b)

	fmt.Println("handleopcallreq返回的结果是", hex.EncodeToString(r.Ret))
	fmt.Println("uuid是", r.UUID)

	go networks.TcpDial(b1, global.Ip_nodeTable[m.FromShardId][m.FromNodeId])

}

func (p *PbftConsensusNode) handleOpCallRes(content []byte) {
	m := new(message.OpCallRes)
	err := json.Unmarshal(content, m)
	if err != nil {
		fmt.Println(err)
	}

	m1 := new(state.OpCallRes)
	m1.Ret = m.Ret
	m1.Journal = m.Journal
	m1.Err = m.Err

	state.OpCallResMapLock.Lock()
	state.OpCallResMap[m.Id] = *m1
	state.OpCallResMapLock.Unlock()

}
func (p *PbftConsensusNode) handleOpStaticCallRes(content []byte) {
	m := new(message.OpStaticCallRes)
	err := json.Unmarshal(content, m)
	if err != nil {
		fmt.Println(err)
	}

	m1 := new(state.OpStaticCallRes)
	m1.Ret = m.Ret

	m1.Err = m.Err

	state.OpStaticCallResMapLock.Lock()
	state.OpStaticCallResMap[m.Id] = *m1
	state.OpStaticCallResMapLock.Unlock()

}
func (p *PbftConsensusNode) handleOpDelegateCallReq(content []byte) {
	msg := new(message.OpDelegateCallReq)
	err := json.Unmarshal(content, msg)
	if err != nil {
		log.Panic(err)
	}

	statedb, _, _ := state.New(global2.Root, p.CurChain.Statedb)
	decodeString, _ := hex.DecodeString(msg.Address)

	CodeHash := statedb.GetCodeHash(common.Address(decodeString))
	Code := statedb.GetCode(common.Address(decodeString))

	m := new(message.OpDelegateCallRes)
	m.Address = msg.Address
	m.CodeHash = CodeHash
	m.Code = Code

	b, _ := json.Marshal(m)
	m1 := message.MergeMessage(message.COpDelegateCallRes, b)
	go networks.TcpDial(m1, p.ip_nodeTable[msg.FromShardId][msg.FromNodeId])
}

func (p *PbftConsensusNode) handleOpDelegateCallRes(content []byte) {
	m := new(message.OpDelegateCallRes)
	err := json.Unmarshal(content, m)
	if err != nil {
		log.Panic(err)
	}
	m1 := new(global.OpDelegatecallRes)
	m1.CodeHash = m.CodeHash
	m1.Code = m.Code
	global.OpDelegatecallResMapLock.Lock()
	global.OpDelegatecallResMap[m.Address] = *m1
	global.OpDelegatecallResMapLock.Unlock()
}

func (p *PbftConsensusNode) handleOpCallCodeReq(content []byte) {
	msg := new(message.OpCallCodeReq)
	err := json.Unmarshal(content, msg)
	if err != nil {
		log.Panic(err)
	}

	statedb, _, _ := state.New(global2.Root, p.CurChain.Statedb)
	decodeString, _ := hex.DecodeString(msg.Address)

	CodeHash := statedb.GetCodeHash(common.Address(decodeString))
	Code := statedb.GetCode(common.Address(decodeString))

	m := new(message.OpCallCodeRes)
	m.Address = msg.Address
	m.CodeHash = CodeHash
	m.Code = Code

	b, _ := json.Marshal(m)
	m1 := message.MergeMessage(message.COpCallCodeRes, b)
	go networks.TcpDial(m1, p.ip_nodeTable[msg.FromShardId][msg.FromNodeId])
}

func (p *PbftConsensusNode) handleOpCallCodeRes(content []byte) {
	m := new(message.OpCallCodeRes)
	err := json.Unmarshal(content, m)
	if err != nil {
		log.Panic(err)
	}
	m1 := new(global.OpCallCodeRes)
	m1.CodeHash = m.CodeHash
	m1.Code = m.Code
	global.OpCallCodeResMapLock.Lock()
	global.OpCallCodeResMap[m.Address] = *m1
	global.OpCallCodeResMapLock.Unlock()
}

func (p *PbftConsensusNode) handleOpbanlanceReq(content []byte) {
	msg := new(message.OpBalanceReq)
	err := json.Unmarshal(content, msg)
	if err != nil {
		log.Panic(err)
	}

	statedb, _, _ := state.New(global2.Root, p.CurChain.Statedb)
	decodeString, _ := hex.DecodeString(msg.Address)
	if !statedb.Exist(common.Address(decodeString)) {
		statedb.CreateAccount(common.Address(decodeString))
	}
	balance := statedb.GetBalance(common.Address(decodeString))

	m := new(message.OpBalanceRes)
	m.Address = msg.Address
	m.Balance = balance.Uint64()

	b, _ := json.Marshal(m)
	m1 := message.MergeMessage(message.COpbalanceRes, b)
	go networks.TcpDial(m1, p.ip_nodeTable[msg.FromShardId][msg.FromNodeId])
}

func (p *PbftConsensusNode) handleOpbanlanceRes(content []byte) {
	m := new(message.OpBalanceRes)
	err := json.Unmarshal(content, m)
	if err != nil {
		log.Panic(err)
	}
	global.OpBalanceResMapLock.Lock()
	global.OpBalanceResMap[m.Address] = m.Balance
	global.OpBalanceResMapLock.Unlock()
}

func (p *PbftConsensusNode) handleClientRequest(con net.Conn) {
	defer con.Close()
	clientReader := bufio.NewReader(con)
	for {
		clientRequest, err := clientReader.ReadBytes('\n')
		if p.getStopSignal() {
			return
		}
		switch err {
		case nil:
			p.tcpPoolLock.Lock()
			p.handleMessage(clientRequest)
			p.tcpPoolLock.Unlock()
		case io.EOF:
			log.Println("client closed the connection by terminating the process")
			return
		default:
			log.Printf("error: %v\n", err)
			return
		}
	}
}

func (p *PbftConsensusNode) TcpListen() {
	ln, err := net.Listen("tcp", p.RunningNode.IPaddr)
	p.tcpln = ln
	if err != nil {
		log.Panic(err)
	}
	for {
		conn, err := p.tcpln.Accept()
		if err != nil {
			return
		}
		go p.handleClientRequest(conn)
	}
}

// listen to the request
func (p *PbftConsensusNode) OldTcpListen() {
	ipaddr, err := net.ResolveTCPAddr("tcp", p.RunningNode.IPaddr)
	if err != nil {
		log.Panic(err)
	}
	ln, err := net.ListenTCP("tcp", ipaddr)
	p.tcpln = ln
	if err != nil {
		log.Panic(err)
	}
	p.pl.Plog.Printf("S%dN%d begins listening：%s\n", p.ShardID, p.NodeID, p.RunningNode.IPaddr)

	for {
		if p.getStopSignal() {
			p.closePbft()
			return
		}
		conn, err := p.tcpln.Accept()
		if err != nil {
			log.Panic(err)
		}
		b, err := io.ReadAll(conn)
		if err != nil {
			log.Panic(err)
		}
		p.handleMessage(b)
		conn.(*net.TCPConn).SetLinger(0)
		defer conn.Close()
	}
}

func (p *PbftConsensusNode) handleQueryAcc(content []byte) {
	msg := new(message.QueryAccount)
	err := json.Unmarshal(content, msg)
	if err != nil {
		log.Panic(err)
	}

	statedb, _ := state.New2(global2.Root, p.CurChain.Statedb)
	decodeString, _ := hex.DecodeString(msg.Account)
	balance := statedb.GetBalance(common.Address(decodeString))
	code := statedb.GetCode(common.Address(decodeString))

	//if sn =="new"{
	//	var E1 error
	//	global2.Root, E1 = statedb.Commit(0, false)
	//	if E1 != nil {
	//		fmt.Println(E1)
	//	}
	//	err1 := p.CurChain.Triedb.Commit(global2.Root, false)
	//	if err1 != nil {
	//		fmt.Println(err1)
	//	}
	//	global3.Lock.Lock()
	//	global3.GlobalStateDB = nil
	//	global3.Lock.Unlock()
	//}

	m := new(message.AccountRes)
	m.Account = msg.Account
	m.Balance = balance.Uint64()
	m.Code = code
	b, _ := json.Marshal(m)
	m1 := message.MergeMessage(message.CResponseAccount, b)
	go networks.TcpDial(m1, p.ip_nodeTable[msg.FromShardID][msg.FromNodeID])

}

func (p *PbftConsensusNode) handleResponseAcc(content []byte) {
	msg := new(message.AccountRes)
	err := json.Unmarshal(content, msg)
	if err != nil {
		log.Panic(err)
	}
	fmt.Printf("当前节点账户是%s,获得的出块奖励是%d\n", msg.Account, msg.Balance)

	m := new(message.NodeInfo)
	m.NodeId = params.NodeID
	m.ShardId = params.ShardID
	m.NodeAccount = global.NodeAccount
	m.NodeBalance = msg.Balance
	m.NodeIP = params.IPmap_nodeTable[params.ShardID][params.NodeID]
	m.NodePublicKey = global.NodePublicKey

	b, _ := json.Marshal(m)
	m1 := message.MergeMessage(message.CNodeInfo, b)
	go networks.TcpDial(m1, p.ip_nodeTable[params.DeciderShard][0])

}

func (p *PbftConsensusNode) handleQueryNodeAcc(content []byte) {
	msg := new(message.QueryNodeAccount)
	err := json.Unmarshal(content, msg)
	if err != nil {
		log.Panic(err)
	}

	m := new(message.NodeAccount)
	m.NodeId = params.NodeID
	m.Account = global.NodeAccount
	b, _ := json.Marshal(m)
	sendmsg := message.MergeMessage(message.CResponseNodeAccount, b)
	go networks.TcpDial(sendmsg, p.ip_nodeTable[params.ShardID][msg.FromNodeID])
}

func (p *PbftConsensusNode) handleResponseNodeAcc(content []byte) {
	msg := new(message.NodeAccount)
	err := json.Unmarshal(content, msg)
	if err != nil {
		log.Panic(err)
	}
	global.NodeAccountMapLock.Lock()
	global.NodeAccountMap[uint(msg.NodeId)] = msg.Account
	global.NodeAccountMapLock.Unlock()
	global.NodeAccChan <- 1
}
func (p *PbftConsensusNode) handleRequestAccountState(content []byte) {
	cras := new(message.ClientRequestAccountState)
	err := json.Unmarshal(content, cras)
	if err != nil {
		log.Panic(err)
	}

	as, shardId, blockHeight, blockHash, stroot := p.CurChain.FetchAccounts2([]string{cras.AccountAddr})

	// generate message and send it to client
	asc := &message.AccountStateForClient{
		AccountAddr:   cras.AccountAddr,
		AccountState:  as[0],
		ShardID:       shardId,
		BlockHeight:   blockHeight,
		BlockHash:     blockHash,
		StateTrieRoot: stroot,
	}

	ascByte, err := json.Marshal(asc)
	if err != nil {
		log.Panic(err)
	}
	send_msg := message.MergeMessage(message.CAccountStateForClient, ascByte)

	// send message
	go networks.TcpDial(send_msg, cras.ClientIp)
}

// TODO
func (p *PbftConsensusNode) handleRequestTransaction(content []byte) {
	crtx := new(message.ClientRequestTransaction)
	err := json.Unmarshal(content, crtx)
	if err != nil {
		log.Panic(err)
	}

	txc := &message.AccountTransactionsForClient{
		AccountAddr: crtx.AccountAddr,
		ShardID:     int(p.CurChain.ChainConfig.ShardID),
		Txs:         nil,
	}

	txs := make([]*core.Transaction, 0)
	nowheight := p.CurChain.CurrentBlock.Header.Number
	nowblockHash := p.CurChain.CurrentBlock.Hash

	for ; nowheight > 0; nowheight-- {
		block, err1 := p.CurChain.Storage.GetBlock(nowblockHash)
		if err1 != nil {
			fmt.Println(err1)
			break
		}
		//triedb := trie.NewDatabase(rawdb.NewMemoryDatabase())
		//transactionTree := trie.NewEmpty(triedb)
		//for _, tx := range block.Body {
		//	transactionTree.Update(tx.TxHash, tx.Encode())
		//}
		//if !bytes.Equal(transactionTree.Hash().Bytes(), block.Header.TxRoot) {
		//	fmt.Println("transaction hash does not match block header hash")
		//	break
		//}

		for _, tx := range block.Body {
			if tx.Isbrokertx1 || tx.Isbrokertx2 {
				if tx.OriginalSender == crtx.AccountAddr {
					txs = append(txs, tx)
				} else if tx.FinalRecipient == crtx.AccountAddr {
					txs = append(txs, tx)
				}
			} else {
				if tx.Sender == crtx.AccountAddr {

				} else if tx.Recipient == crtx.AccountAddr {

				}
			}
		}

		nowblockHash = block.Header.ParentBlockHash
	}

	if uint64(utils.Addr2Shard(crtx.AccountAddr)) == p.CurChain.ChainConfig.ShardID {

	} else {

	}

	txc.Txs = txs

	txcByte, err := json.Marshal(txc)
	if err != nil {
		log.Panic(err)
	}
	send_msg := message.MergeMessage(message.CAccountTransactionForClient, txcByte)

	// send message
	go networks.TcpDial(send_msg, crtx.ClientIp)

}

// when received stop
func (p *PbftConsensusNode) WaitToStop() {
	p.pl.Plog.Println("handling stop message")
	p.stopLock.Lock()
	p.stop = true
	p.stopLock.Unlock()
	if p.NodeID == p.view {
		p.pStop <- 1
	}
	networks.CloseAllConnInPool()
	p.tcpln.Close()
	p.closePbft()
	p.pl.Plog.Println("handled stop message")
}

func (p *PbftConsensusNode) getStopSignal() bool {
	p.stopLock.Lock()
	defer p.stopLock.Unlock()
	return p.stop
}

// close the pbft
func (p *PbftConsensusNode) closePbft() {
	p.CurChain.CloseBlockChain()
}
