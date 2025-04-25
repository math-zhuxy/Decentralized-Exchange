package vm

import (
	"blockEmulator/global"
	"blockEmulator/message"
	"blockEmulator/networks"
	params2 "blockEmulator/params"
	"blockEmulator/utils"
	"blockEmulator/vm/state"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"strconv"
	"time"

	"blockEmulator/vm/tracing"
	"errors"
	"math/big"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	//"github.com/ethereum/go-ethereum/params"
	"blockEmulator/vm/params"
	"github.com/holiman/uint256"
)

type (
	// CanTransferFunc is the signature of a transfer guard function
	CanTransferFunc func(StateDB, common.Address, *uint256.Int) bool
	// TransferFunc is the signature of a transfer function
	TransferFunc func(StateDB, common.Address, common.Address, *uint256.Int)
	// GetHashFunc returns the n'th block hash in the blockchain
	// and is used by the BLOCKHASH EVM op code.
	GetHashFunc func(uint64) common.Hash
)

func (evm *EVM) precompile(addr common.Address) (PrecompiledContract, bool) {
	p, ok := evm.precompiles[addr]
	return p, ok
}

// BlockContext provides the EVM with auxiliary information. Once provided
// it shouldn't be modified.
type BlockContext struct {
	// CanTransfer returns whether the account contains
	// sufficient ether to transfer the value
	CanTransfer CanTransferFunc
	// Transfer transfers ether from one account to the other
	Transfer TransferFunc
	// GetHash returns the hash corresponding to n
	GetHash GetHashFunc

	// Block information
	Coinbase    common.Address // Provides information for COINBASE
	GasLimit    uint64         // Provides information for GASLIMIT
	BlockNumber *big.Int       // Provides information for NUMBER
	Time        uint64         // Provides information for TIME
	Difficulty  *big.Int       // Provides information for DIFFICULTY
	BaseFee     *big.Int       // Provides information for BASEFEE (0 if vm runs with NoBaseFee flag and 0 gas price)
	BlobBaseFee *big.Int       // Provides information for BLOBBASEFEE (0 if vm runs with NoBaseFee flag and 0 blob gas price)
	Random      *common.Hash   // Provides information for PREVRANDAO
}

// TxContext provides the EVM with information about a transaction.
// All fields can change between transactions.
type TxContext struct {
	// Message information
	Origin       common.Address      // Provides information for ORIGIN
	GasPrice     *big.Int            // Provides information for GASPRICE (and is used to zero the basefee if NoBaseFee is set)
	BlobHashes   []common.Hash       // Provides information for BLOBHASH
	BlobFeeCap   *big.Int            // Is used to zero the blobbasefee if NoBaseFee is set
	AccessEvents *state.AccessEvents // Capture all state accesses for this tx
}

// EVM is the Ethereum Virtual Machine base object and provides
// the necessary tools to run a contract on the given state with
// the provided context. It should be noted that any error
// generated through any of the calls should be considered a
// revert-state-and-consume-all-gas operation, no checks on
// specific errors should ever be performed. The interpreter makes
// sure that any errors generated are to be considered faulty code.
//
// The EVM should never be reused and is not thread safe.
type EVM struct {
	// Context provides auxiliary blockchain related information
	Context BlockContext
	TxContext
	// StateDB gives access to the underlying state
	StateDB StateDB
	// Depth is the current call stack
	depth int

	// chainConfig contains information about the current chain
	chainConfig *params.ChainConfig
	// chain rules contains the chain rules for the current epoch
	chainRules params.Rules
	// virtual machine configuration options used to initialise the
	// evm.
	Config Config
	// global (to this context) ethereum virtual machine
	// used throughout the execution of the tx.
	interpreter *EVMInterpreter
	// abort is used to abort the EVM calling operations
	abort atomic.Bool
	// callGasTemp holds the gas available for the current call. This is needed because the
	// available gas is calculated in gasCall* according to the 63/64 rule and later
	// applied in opCall*.
	callGasTemp uint64
	// precompiles holds the precompiled contracts for the current epoch
	precompiles map[common.Address]PrecompiledContract
}

// NewEVM returns a new EVM. The returned EVM is not thread safe and should
// only ever be used *once*.
func NewEVM(blockCtx BlockContext, txCtx TxContext, statedb StateDB, chainConfig *params.ChainConfig, config Config) *EVM {
	evm := &EVM{
		Context:     blockCtx,
		TxContext:   txCtx,
		StateDB:     statedb,
		Config:      config,
		chainConfig: chainConfig,
		//chainRules:  chainConfig.Rules(blockCtx.BlockNumber, blockCtx.Random != nil, blockCtx.Time),
	}
	evm.precompiles = activePrecompiledContracts(evm.chainRules)
	evm.interpreter = NewEVMInterpreter(evm)
	return evm
}

// SetPrecompiles sets the precompiled contracts for the EVM.
// This method is only used through RPC calls.
// It is not thread-safe.
func (evm *EVM) SetPrecompiles(precompiles PrecompiledContracts) {
	evm.precompiles = precompiles
}

// Reset resets the EVM with a new transaction context.Reset
// This is not threadsafe and should only be done very cautiously.
func (evm *EVM) Reset(txCtx TxContext, statedb StateDB) {
	//if evm.chainRules.IsEIP4762 {
	//	txCtx.AccessEvents = state.NewAccessEvents(statedb.PointCache())
	//}
	evm.TxContext = txCtx
	evm.StateDB = statedb
}

// Cancel cancels any running EVM operation. This may be called concurrently and
// it's safe to be called multiple times.
func (evm *EVM) Cancel() {
	evm.abort.Store(true)
}

// Cancelled returns true if Cancel has been called
func (evm *EVM) Cancelled() bool {
	return evm.abort.Load()
}

// Interpreter returns the current interpreter
func (evm *EVM) Interpreter() *EVMInterpreter {
	return evm.interpreter
}

// Call executes the contract associated with the addr with the given input as
// parameters. It also handles any necessary value transfer required and takes
// the necessary steps to create accounts and reverses the state in case of an
// execution error or failed value transfer.
func (evm *EVM) Call(caller ContractRef, addr common.Address, input []byte, gas uint64, value *uint256.Int,UUID string) (ret []byte, leftOverGas uint64, err error) {
	if evm.depth > int(params.CallCreateDepth) {
		return nil, gas, ErrDepth
	}
	if !value.IsZero() && !evm.Context.CanTransfer(evm.StateDB, caller.Address(), value) {
		return nil, gas, ErrInsufficientBalance
	}
	snapshot := evm.StateDB.Snapshot()
	p, isPrecompile := evm.precompile(addr)

	//if !evm.StateDB.Exist(addr) {
	//	evm.StateDB.CreateAccount(addr)
	//}
	//evm.Context.Transfer(evm.StateDB, caller.Address(), addr, value)

	t:=hex.EncodeToString(caller.Address().Bytes())

	toString := hex.EncodeToString(addr[:])
	fmt.Println("caller:",t,", Get_PartitionMap(caller): ", Get_PartitionMap(t))
	fmt.Println("global.ShardID:",global.ShardID,",toString:",toString,", Get_PartitionMap(toString): ", Get_PartitionMap(toString))
	fmt.Println("value:",value.ToBig().String())
	if global.ShardID != Get_PartitionMap(toString) {

		Transfer1(evm.StateDB, caller.Address(), addr, value)
	} else {
		if !evm.StateDB.Exist(addr) {
			evm.StateDB.CreateAccount(addr)
		}
		Transfer(evm.StateDB, caller.Address(), addr, value)
	}



	if isPrecompile {
		ret, gas, err = RunPrecompiledContract(p, input, gas, evm.Config.Tracer)
	} else {
		if global.ShardID != Get_PartitionMap(toString) {
			fmt.Println("开始跨片call,目标片：",Get_PartitionMap(toString))
			fmt.Println("value为"+strconv.Itoa(int(value.Uint64())))
			u :=uuid.New().String()
			ms:= new(message.OpCallReq)
			ms.Caller = caller.Address().Bytes()
			ms.Addr = addr
			ms.Value = value.Uint64()
			ms.Gas = gas
			ms.Input = input
			ms.FromShardId = global.ShardID
			ms.FromNodeId = global.NodeID
			ms.Id = u
			ms.UUID = UUID

			b,e := json.Marshal(ms)
			if e != nil {
				fmt.Println(e)
			}
			b1:=message.MergeMessage(message.COpCallReq,b)
			go networks.TcpDial(b1,global.Ip_nodeTable[Get_PartitionMap(toString)][0])

			now:=time.Now()
			var res state.OpCallRes
			var ok bool
			for {
				//如果等待结果的时间超过了 5秒，则认为读取失败，将结果设置为 0，跳出循环
				if time.Since(now).Seconds() > 5 {
					res =state.OpCallRes{}
					res.Err="timeout"
					fmt.Println("opcall超时")
					break
				}
				//为了保证多协程环境下的原子性和可见性，需要加锁
				state.OpCallResMapLock.Lock()
				//从用于存储返回结果的哈希表中尝试读取返回的响应结果
				res, ok = state.OpCallResMap[u]
				//解锁
				state.OpCallResMapLock.Unlock()
				if ok {
					//读取成功的情况
					//为了保证多协程环境下的原子性和可见性，需要加锁
					state.OpCallResMapLock.Lock()
					//从用于存储返回结果的哈希表中删除返回的响应结果
					delete(state.OpCallResMap, u)
					//解锁
					state.OpCallResMapLock.Unlock()
					//读取成功,跳出循环
					break
				}
				//读取失败的情况，为了防止 CPU占用率过高，使用 sleep将协程阻塞一段时间
				time.Sleep(1 * time.Millisecond)
			}

			if res.Err!=""{
				fmt.Println("跨片opcall错误：",res.Err)
				err = errors.New(res.Err)
			} else {
				fmt.Println("跨片opcall成功：", hex.EncodeToString(res.Ret))
				ret = res.Ret
				err = nil

				for _, item := range res.Journal {
					evm.StateDB.(*state.StateDB).GetJournal().Entries = append(evm.StateDB.(*state.StateDB).GetJournal().Entries, item.OriginalObj)
				}
			}
			gas = res.Gas



		} else {
		code := evm.StateDB.GetCode(addr)
		//if witness := evm.StateDB.Witness(); witness != nil {
		//	witness.AddCode(code)
		//}
		if len(code) == 0 {
			ret, err = nil, nil // gas is unchanged
		} else {
			addrCopy := addr
			contract := NewContract(caller, AccountRef(addrCopy), value, gas)
			contract.UUID = UUID
			contract.SetCallCode(&addrCopy, evm.StateDB.GetCodeHash(addrCopy), code)
			ret, err = evm.interpreter.Run(contract, input, false)
			gas = contract.Gas
		}
	}
	}
	if err != nil {
		evm.StateDB.RevertToSnapshot(snapshot)
		if err != ErrExecutionReverted {
			if evm.Config.Tracer != nil && evm.Config.Tracer.OnGasChange != nil {
				evm.Config.Tracer.OnGasChange(gas, 0, tracing.GasChangeCallFailedExecution)
			}

			gas = 0
		}
	}
	return ret, gas, err
}

func Transfer(db StateDB, sender, recipient common.Address, amount *uint256.Int) {
	db.SubBalance(sender, amount, tracing.BalanceChangeTransfer)
	db.AddBalance(recipient, amount, tracing.BalanceChangeTransfer)
}

func Transfer1(db StateDB, sender, recipient common.Address, amount *uint256.Int) {
	db.SubBalance(sender, amount, tracing.BalanceChangeTransfer)
	//db.AddBalance(recipient, amount, tracing.BalanceChangeTransfer)
}

func Transfer2(db StateDB, sender, recipient common.Address, amount *uint256.Int) {
	//db.SubBalance(sender, amount, tracing.BalanceChangeTransfer)
	db.AddBalance(recipient, amount, tracing.BalanceChangeTransfer)
}

// 专门用于opCall指令进行调用
func (evm *EVM) OpCall(caller ContractRef, addr common.Address, input []byte, gas uint64, value *uint256.Int) (ret []byte, leftOverGas uint64, err error) {
	if evm.depth > int(params.CallCreateDepth) {
		return nil, gas, ErrDepth
	}
	if !value.IsZero() && !evm.Context.CanTransfer(evm.StateDB, caller.Address(), value) {
		fmt.Println("opcall错误",ErrInsufficientBalance.Error())
		return nil, gas, ErrInsufficientBalance
	}
	snapshot := evm.StateDB.Snapshot()
	p, isPrecompile := evm.precompile(addr)

	//if !evm.StateDB.Exist(addr) {
	//	evm.StateDB.CreateAccount(addr)
	//}

	toString := hex.EncodeToString(addr[:])
	if global.ShardID != Get_PartitionMap(toString) {
		Transfer1(evm.StateDB, caller.Address(), addr, value)
	} else {
		if !evm.StateDB.Exist(addr) {
			evm.StateDB.CreateAccount(addr)
		}
		Transfer(evm.StateDB, caller.Address(), addr, value)
	}

	if isPrecompile {
		ret, gas, err = RunPrecompiledContract(p, input, gas, evm.Config.Tracer)
	} else {

		if global.ShardID != Get_PartitionMap(toString) {
			fmt.Println("开始跨片call,目标片：",Get_PartitionMap(toString))
			u :=uuid.New().String()
			ms:= new(message.OpCallReq)
			ms.Caller = caller.Address().Bytes()
			ms.Addr = addr
			ms.Value = value.Uint64()
			ms.Gas = gas
			ms.Input = input
			ms.FromShardId = global.ShardID
			ms.FromNodeId = global.NodeID
			ms.Id = u
			ms.UUID = caller.(*Contract).UUID

			b,e := json.Marshal(ms)
			if e != nil {
				fmt.Println(e)
			}
			b1:=message.MergeMessage(message.COpCallReq,b)
			go networks.TcpDial(b1,global.Ip_nodeTable[Get_PartitionMap(toString)][0])

			now:=time.Now()
			var res state.OpCallRes
			var ok bool
			for {
				//如果等待结果的时间超过了 5秒，则认为读取失败，将结果设置为 0，跳出循环
				if time.Since(now).Seconds() > 60 {
					res =state.OpCallRes{}
					res.Err="timeout"
					fmt.Println("opcall超时")
					break
				}
				//为了保证多协程环境下的原子性和可见性，需要加锁
				state.OpCallResMapLock.Lock()
				//从用于存储返回结果的哈希表中尝试读取返回的响应结果
				res, ok = state.OpCallResMap[u]
				//解锁
				state.OpCallResMapLock.Unlock()
				if ok {
					//读取成功的情况
					//为了保证多协程环境下的原子性和可见性，需要加锁
					state.OpCallResMapLock.Lock()
					//从用于存储返回结果的哈希表中删除返回的响应结果
					delete(state.OpCallResMap, u)
					//解锁
					state.OpCallResMapLock.Unlock()
					//读取成功,跳出循环
					break
				}
				//读取失败的情况，为了防止 CPU占用率过高，使用 sleep将协程阻塞一段时间
				time.Sleep(1 * time.Millisecond)
			}

			if res.Err!=""{
				fmt.Println("跨片opcall错误：",res.Err)
				err = errors.New(res.Err)
			} else {
				fmt.Println("跨片opcall成功：", hex.EncodeToString(res.Ret))
				ret = res.Ret
				err = nil

				for _, item := range res.Journal {
					evm.StateDB.(*state.StateDB).GetJournal().Entries = append(evm.StateDB.(*state.StateDB).GetJournal().Entries, item.OriginalObj)
				}
			}
			gas = res.Gas



		} else {
			code := evm.StateDB.GetCode(addr)
			if witness := evm.StateDB.Witness(); witness != nil {
				witness.AddCode(code)
			}
			if len(code) == 0 {
				ret, err = nil, nil // gas is unchanged
			} else {
				addrCopy := addr
				contract := NewContract(caller, AccountRef(addrCopy), value, gas)
				contract.SetCallCode(&addrCopy, evm.StateDB.GetCodeHash(addrCopy), code)
				ret, err = evm.interpreter.Run(contract, input, false)
				gas = contract.Gas
				if err != nil {
					fmt.Println("片内opcall错误：",err.Error())
				} else {
					fmt.Println("片内opcall成功：",hex.EncodeToString(ret))
				}
			}
		}

	}
	if err != nil {
		fmt.Println("opcall错误,",err.Error())
		evm.StateDB.RevertToSnapshot(snapshot)
		if err != ErrExecutionReverted {
			if evm.Config.Tracer != nil && evm.Config.Tracer.OnGasChange != nil {
				evm.Config.Tracer.OnGasChange(gas, 0, tracing.GasChangeCallFailedExecution)
			}

			gas = 0
		}
	}
	return ret, gas, err
}

// CallCode executes the contract associated with the addr with the given input
// as parameters. It also handles any necessary value transfer required and takes
// the necessary steps to create accounts and reverses the state in case of an
// execution error or failed value transfer.
//
// CallCode differs from Call in the sense that it executes the given address'
// code with the caller as context.
func (evm *EVM) CallCode(caller ContractRef, addr common.Address, input []byte, gas uint64, value *uint256.Int) (ret []byte, leftOverGas uint64, err error) {
	// Invoke tracer hooks that signal entering/exiting a call frame
	if evm.Config.Tracer != nil {
		evm.captureBegin(evm.depth, CALLCODE, caller.Address(), addr, input, gas, value.ToBig())
		defer func(startGas uint64) {
			evm.captureEnd(evm.depth, startGas, leftOverGas, ret, err)
		}(gas)
	}
	// Fail if we're trying to execute above the call depth limit
	if evm.depth > int(params.CallCreateDepth) {
		return nil, gas, ErrDepth
	}
	// Fail if we're trying to transfer more than the available balance
	// Note although it's noop to transfer X ether to caller itself. But
	// if caller doesn't have enough balance, it would be an error to allow
	// over-charging itself. So the check here is necessary.
	if !evm.Context.CanTransfer(evm.StateDB, caller.Address(), value) {
		return nil, gas, ErrInsufficientBalance
	}
	var snapshot = evm.StateDB.Snapshot()

	// It is allowed to call precompiles, even via delegatecall
	if p, isPrecompile := evm.precompile(addr); isPrecompile {
		ret, gas, err = RunPrecompiledContract(p, input, gas, evm.Config.Tracer)
	} else {
		addrCopy := addr
		// Initialise a new contract and set the code that is to be used by the EVM.
		// The contract is a scoped environment for this execution context only.
		contract := NewContract(caller, AccountRef(caller.Address()), value, gas)
		if witness := evm.StateDB.Witness(); witness != nil {
			witness.AddCode(evm.StateDB.GetCode(addrCopy))
		}
		address := addrCopy
		toString := hex.EncodeToString(address[:])
		var res global.OpCallCodeRes
		if Get_PartitionMap(toString) != global.ShardID {
			//跨片读取
			msg := new(message.OpCallCodeReq)
			msg.Address = toString
			msg.FromShardId = global.ShardID
			msg.FromNodeId = global.NodeID
			b, _ := json.Marshal(msg)
			b1 := message.MergeMessage(message.COpCallCodeReq, b)
			go networks.TcpDial(b1, global.Ip_nodeTable[Get_PartitionMap(toString)][0])

			var ok bool
			now := time.Now()
			for {
				if time.Since(now).Seconds() > 5 {
					res = *new(global.OpCallCodeRes)
					break
				}
				global.OpCallCodeResMapLock.Lock()
				res, ok = global.OpCallCodeResMap[toString]
				global.OpCallCodeResMapLock.Unlock()
				if ok {
					global.OpCallCodeResMapLock.Lock()
					delete(global.OpCallCodeResMap, toString)
					global.OpCallCodeResMapLock.Unlock()
					break
				}
				time.Sleep(1 * time.Millisecond)
			}
		} else {
			//本地读取
			res = *new(global.OpCallCodeRes)
			res.CodeHash = evm.StateDB.GetCodeHash(addrCopy)
			res.Code = evm.StateDB.GetCode(addrCopy)
		}

		//contract.SetCallCode(&addrCopy, evm.StateDB.GetCodeHash(addrCopy), evm.StateDB.GetCode(addrCopy))
		contract.SetCallCode(&addrCopy, res.CodeHash, res.Code)
		ret, err = evm.interpreter.Run(contract, input, false)
		gas = contract.Gas
	}
	if err != nil {
		fmt.Println("调用callcode错误,",err.Error())
		evm.StateDB.RevertToSnapshot(snapshot)
		if err != ErrExecutionReverted {
			if evm.Config.Tracer != nil && evm.Config.Tracer.OnGasChange != nil {
				evm.Config.Tracer.OnGasChange(gas, 0, tracing.GasChangeCallFailedExecution)
			}

			gas = 0
		}
	}
	return ret, gas, err
}

// DelegateCall executes the contract associated with the addr with the given input
// as parameters. It reverses the state in case of an execution error.
//
// DelegateCall differs from CallCode in the sense that it executes the given address'
// code with the caller as context and the caller is set to the caller of the caller.
func (evm *EVM) DelegateCall(caller ContractRef, addr common.Address, input []byte, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	// Invoke tracer hooks that signal entering/exiting a call frame
	if evm.Config.Tracer != nil {
		// NOTE: caller must, at all times be a contract. It should never happen
		// that caller is something other than a Contract.
		parent := caller.(*Contract)
		// DELEGATECALL inherits value from parent call
		evm.captureBegin(evm.depth, DELEGATECALL, caller.Address(), addr, input, gas, parent.value.ToBig())
		defer func(startGas uint64) {
			evm.captureEnd(evm.depth, startGas, leftOverGas, ret, err)
		}(gas)
	}
	// Fail if we're trying to execute above the call depth limit
	if evm.depth > int(params.CallCreateDepth) {
		return nil, gas, ErrDepth
	}
	var snapshot = evm.StateDB.Snapshot()

	// It is allowed to call precompiles, even via delegatecall
	if p, isPrecompile := evm.precompile(addr); isPrecompile {
		ret, gas, err = RunPrecompiledContract(p, input, gas, evm.Config.Tracer)
	} else {
		addrCopy := addr
		// Initialise a new contract and make initialise the delegate values
		contract := NewContract(caller, AccountRef(caller.Address()), nil, gas).AsDelegate()
		if witness := evm.StateDB.Witness(); witness != nil {
			witness.AddCode(evm.StateDB.GetCode(addrCopy))
		}

		//delegatecall在调用者的上下文执行，故只需要实现跨片读取被调用的合约代码，在本地执行
		address := addrCopy
		toString := hex.EncodeToString(address[:])
		var res global.OpDelegatecallRes
		if Get_PartitionMap(toString) != global.ShardID {
			//跨片读取
			msg := new(message.OpDelegateCallReq)
			msg.Address = toString
			msg.FromShardId = global.ShardID
			msg.FromNodeId = global.NodeID
			b, _ := json.Marshal(msg)
			b1 := message.MergeMessage(message.COpDelegateCallReq, b)
			go networks.TcpDial(b1, global.Ip_nodeTable[Get_PartitionMap(toString)][0])

			var ok bool
			now := time.Now()
			for {
				if time.Since(now).Seconds() > 5 {
					res = *new(global.OpDelegatecallRes)
					break
				}
				global.OpDelegatecallResMapLock.Lock()
				res, ok = global.OpDelegatecallResMap[toString]
				global.OpDelegatecallResMapLock.Unlock()
				if ok {
					global.OpDelegatecallResMapLock.Lock()
					delete(global.OpDelegatecallResMap, toString)
					global.OpDelegatecallResMapLock.Unlock()
					break
				}
				time.Sleep(1 * time.Millisecond)
			}
		} else {
			//本地读取
			res = *new(global.OpDelegatecallRes)
			res.CodeHash = evm.StateDB.GetCodeHash(addrCopy)
			res.Code = evm.StateDB.GetCode(addrCopy)
		}
		//contract.SetCallCode(&addrCopy, evm.StateDB.GetCodeHash(addrCopy), evm.StateDB.GetCode(addrCopy))
		contract.SetCallCode(&addrCopy, res.CodeHash, res.Code)
		ret, err = evm.interpreter.Run(contract, input, false)
		gas = contract.Gas
	}
	if err != nil {
		fmt.Println("调用delegatecall错误,",err.Error())
		evm.StateDB.RevertToSnapshot(snapshot)
		if err != ErrExecutionReverted {
			if evm.Config.Tracer != nil && evm.Config.Tracer.OnGasChange != nil {
				evm.Config.Tracer.OnGasChange(gas, 0, tracing.GasChangeCallFailedExecution)
			}
			gas = 0
		}
	}
	return ret, gas, err
}

// StaticCall executes the contract associated with the addr with the given input
// as parameters while disallowing any modifications to the state during the call.
// Opcodes that attempt to perform such modifications will result in exceptions
// instead of performing the modifications.
func (evm *EVM) StaticCall(caller ContractRef, addr common.Address, input []byte, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	// Invoke tracer hooks that signal entering/exiting a call frame
	if evm.Config.Tracer != nil {
		evm.captureBegin(evm.depth, STATICCALL, caller.Address(), addr, input, gas, nil)
		defer func(startGas uint64) {
			evm.captureEnd(evm.depth, startGas, leftOverGas, ret, err)
		}(gas)
	}
	// Fail if we're trying to execute above the call depth limit
	if evm.depth > int(params.CallCreateDepth) {
		return nil, gas, ErrDepth
	}
	// We take a snapshot here. This is a bit counter-intuitive, and could probably be skipped.
	// However, even a staticcall is considered a 'touch'. On mainnet, static calls were introduced
	// after all empty accounts were deleted, so this is not required. However, if we omit this,
	// then certain tests start failing; stRevertTest/RevertPrecompiledTouchExactOOG.json.
	// We could change this, but for now it's left for legacy reasons
	var snapshot = evm.StateDB.Snapshot()

	// We do an AddBalance of zero here, just in order to trigger a touch.
	// This doesn't matter on Mainnet, where all empties are gone at the time of Byzantium,
	// but is the correct thing to do and matters on other networks, in tests, and potential
	// future scenarios
	//evm.StateDB.AddBalance(addr, new(uint256.Int), tracing.BalanceChangeTouchAccount)

	if p, isPrecompile := evm.precompile(addr); isPrecompile {
		ret, gas, err = RunPrecompiledContract(p, input, gas, evm.Config.Tracer)
	} else {
		// At this point, we use a copy of address. If we don't, the go compiler will
		// leak the 'contract' to the outer scope, and make allocation for 'contract'
		// even if the actual execution ends on RunPrecompiled above.
		addrCopy := addr
		toString := hex.EncodeToString(addr[:])

		if global.ShardID != Get_PartitionMap(toString) {
			fmt.Println("开始跨片staticcall,目标片：",Get_PartitionMap(toString))
			u :=uuid.New().String()
			ms:= new(message.OpStaticCallReq)
			ms.Caller = caller.Address().Bytes()
			ms.Addr = addr
			ms.Gas = gas
			ms.Input = input
			ms.FromShardId = global.ShardID
			ms.FromNodeId = global.NodeID
			ms.Id = u
			ms.UUID = caller.(*Contract).UUID

			b,e := json.Marshal(ms)
			if e != nil {
				fmt.Println(e)
			}
			b1:=message.MergeMessage(message.COpStaticCallReq,b)
			go networks.TcpDial(b1,global.Ip_nodeTable[Get_PartitionMap(toString)][0])

			now:=time.Now()
			var res state.OpStaticCallRes
			var ok bool
			for {
				//如果等待结果的时间超过了 5秒，则认为读取失败，将结果设置为 0，跳出循环
				if time.Since(now).Seconds() > 5 {
					res =state.OpStaticCallRes{}
					res.Err="timeout"
					fmt.Println("opStaticcall超时")
					break
				}
				//为了保证多协程环境下的原子性和可见性，需要加锁
				state.OpStaticCallResMapLock.Lock()
				//从用于存储返回结果的哈希表中尝试读取返回的响应结果
				res, ok = state.OpStaticCallResMap[u]
				//解锁
				state.OpStaticCallResMapLock.Unlock()
				if ok {
					//读取成功的情况
					//为了保证多协程环境下的原子性和可见性，需要加锁
					state.OpStaticCallResMapLock.Lock()
					//从用于存储返回结果的哈希表中删除返回的响应结果
					delete(state.OpStaticCallResMap, u)
					//解锁
					state.OpStaticCallResMapLock.Unlock()
					//读取成功,跳出循环
					break
				}
				//读取失败的情况，为了防止 CPU占用率过高，使用 sleep将协程阻塞一段时间
				time.Sleep(1 * time.Millisecond)
			}

			if res.Err!=""{
				fmt.Println("跨片opstaticcall错误：",res.Err)
				err = errors.New(res.Err)
			} else {
				fmt.Println("跨片opstaticcall成功：", hex.EncodeToString(res.Ret))
				ret = res.Ret
				err = nil

			}
			gas = res.Gas



		} else {


		// Initialise a new contract and set the code that is to be used by the EVM.
		// The contract is a scoped environment for this execution context only.
		contract := NewContract(caller, AccountRef(addrCopy), new(uint256.Int), gas)
		if witness := evm.StateDB.Witness(); witness != nil {
			witness.AddCode(evm.StateDB.GetCode(addrCopy))
		}
		contract.SetCallCode(&addrCopy, evm.StateDB.GetCodeHash(addrCopy), evm.StateDB.GetCode(addrCopy))
		// When an error was returned by the EVM or when setting the creation code
		// above we revert to the snapshot and consume any gas remaining. Additionally
		// when we're in Homestead this also counts for code storage gas errors.
		ret, err = evm.interpreter.Run(contract, input, true)
		gas = contract.Gas
		}
	}
	if err != nil {
		fmt.Println("调用staticcall错误，",err.Error())
		evm.StateDB.RevertToSnapshot(snapshot)
		if err != ErrExecutionReverted {
			if evm.Config.Tracer != nil && evm.Config.Tracer.OnGasChange != nil {
				evm.Config.Tracer.OnGasChange(gas, 0, tracing.GasChangeCallFailedExecution)
			}

			gas = 0
		}
	}
	return ret, gas, err
}

type codeAndHash struct {
	code []byte
	hash common.Hash
}

func (c *codeAndHash) Hash() common.Hash {
	if c.hash == (common.Hash{}) {
		c.hash = crypto.Keccak256Hash(c.code)
	}
	return c.hash
}

// create creates a new contract using code as deployment code.
func (evm *EVM) create(caller ContractRef, codeAndHash *codeAndHash, gas uint64, value *uint256.Int, address common.Address, typ OpCode,UUID string) (ret []byte, createAddress common.Address, leftOverGas uint64, err error) {
	//if evm.Config.Tracer != nil {
	//	evm.captureBegin(evm.depth, typ, caller.Address(), address, codeAndHash.code, gas, value.ToBig())
	//	defer func(startGas uint64) {
	//		evm.captureEnd(evm.depth, startGas, leftOverGas, ret, err)
	//	}(gas)
	//}
	// Depth check execution. Fail if we're trying to execute above the
	// limit.
	if evm.depth > int(params.CallCreateDepth) {
		fmt.Println("创建合约错误，",ErrDepth.Error())
		return nil, common.Address{}, gas, ErrDepth
	}
	if !evm.Context.CanTransfer(evm.StateDB, caller.Address(), value) {
		fmt.Println("创建合约错误，",ErrInsufficientBalance.Error())
		return nil, common.Address{}, gas, ErrInsufficientBalance
	}
	nonce := evm.StateDB.GetNonce(caller.Address())
	if nonce+1 < nonce {
		fmt.Println("创建合约错误，",ErrNonceUintOverflow.Error())
		return nil, common.Address{}, gas, ErrNonceUintOverflow
	}
	evm.StateDB.SetNonce(caller.Address(), nonce+1)

	// We add this to the access list _before_ taking a snapshot. Even if the
	// creation fails, the access-list change should not be rolled back.
	//if evm.chainRules.IsEIP2929 {
	//	evm.StateDB.AddAddressToAccessList(address)
	//}
	// Ensure there's no existing contract already at the designated address.
	// Account is regarded as existent if any of these three conditions is met:
	// - the nonce is non-zero
	// - the code is non-empty
	// - the storage is non-empty

	toString:=hex.EncodeToString(address[:])
	if Get_PartitionMap(toString) == global.ShardID{
		contractHash := evm.StateDB.GetCodeHash(address)
		storageRoot := evm.StateDB.GetStorageRoot(address)
		if evm.StateDB.GetNonce(address) != 0 ||
			(contractHash != (common.Hash{}) && contractHash != types.EmptyCodeHash) || // non-empty code
			(storageRoot != (common.Hash{}) && storageRoot != types.EmptyRootHash) { // non-empty storage
			//if evm.Config.Tracer != nil && evm.Config.Tracer.OnGasChange != nil {
			//	evm.Config.Tracer.OnGasChange(gas, 0, tracing.GasChangeCallFailedExecution)
			//}
			fmt.Println("片内opcreate错误,",ErrContractAddressCollision.Error())
			return nil, common.Address{}, 0, ErrContractAddressCollision
		}
		// Create a new account on the state only if the object was not present.
		// It might be possible the contract code is deployed to a pre-existent
		// account with non-zero balance.
		snapshot := evm.StateDB.Snapshot()
		if !evm.StateDB.Exist(address) {
			evm.StateDB.CreateAccount(address)
		}
		ib := new(big.Int)
		ib.Add(ib, params2.Init_Balance)
		//evm.StateDB.SubBalance(address, uint256.MustFromBig(ib), tracing.BalanceInilizeByXBZ)
		// CreateContract means that regardless of whether the account previously existed
		// in the state trie or not, it _now_ becomes created as a _contract_ account.
		// This is performed _prior_ to executing the initcode,  since the initcode
		// acts inside that account.
		evm.StateDB.CreateContract(address)

		//if evm.chainRules.IsEIP158 {
		//	evm.StateDB.SetNonce(address, 1)
		//}
		evm.Context.Transfer(evm.StateDB, caller.Address(), address, value)

		// Initialise a new contract and set the code that is to be used by the EVM.
		// The contract is a scoped environment for this execution context only.
		contract := NewContract(caller, AccountRef(address), value, gas)
		contract.SetCodeOptionalHash(&address, codeAndHash)
		contract.IsDeployment = true

		ret, err = evm.InitNewContract(contract, address, value)
		//if err != nil && (evm.chainRules.IsHomestead || err != ErrCodeStoreOutOfGas) {
		if err != nil {
			fmt.Println("片内opcreate错误,",err.Error())
			evm.StateDB.RevertToSnapshot(snapshot)
			if err != ErrExecutionReverted {
				//contract.UseGas(contract.Gas, evm.Config.Tracer, tracing.GasChangeCallFailedExecution)
				//contract.UseGas(contract.Gas)
			}
		}
		return ret, address, contract.Gas, err
	} else {
		snapshot := evm.StateDB.Snapshot()
		m:= new(message.OpCreateReq)
		m.Caller = caller.Address().Bytes()
		m.Addr = address
		m.Gas = gas
		m.Value =value.Uint64()
		m.Code = codeAndHash.code
		m.FromShardId = global.ShardID
		m.FromNodeId = global.NodeID
		m.Id = uuid.New().String()
		//m.UUID = caller.(*Contract).UUID
		m.UUID = UUID

		b,_:=json.Marshal(m)
		b1:= message.MergeMessage(message.COpCreateReq,b)
		go networks.TcpDial(b1,global.Ip_nodeTable[Get_PartitionMap(toString)][0])
		now:=time.Now()
		var res state.OpCreateRes
		var ok bool
		for {
			//如果等待结果的时间超过了 5秒，则认为读取失败，将结果设置为 0，跳出循环
			if time.Since(now).Seconds() > 5 {
				res = *new(state.OpCreateRes)
				res.Err = "timeout"
				fmt.Println("跨片create超时。。。")
				break
			}
			//为了保证多协程环境下的原子性和可见性，需要加锁
			state.OpCreateResMapLock.Lock()
			//从用于存储返回结果的哈希表中尝试读取返回的响应结果
			res, ok = state.OpCreateResMap[m.Id]
			//解锁
			state.OpCreateResMapLock.Unlock()
			if ok {
				//读取成功的情况
				//为了保证多协程环境下的原子性和可见性，需要加锁
				state.OpCreateResMapLock.Lock()
				//从用于存储返回结果的哈希表中删除返回的响应结果
				delete(state.OpCreateResMap, m.Id)
				//解锁
				state.OpCreateResMapLock.Unlock()
				//读取成功,跳出循环
				break
			}
			//读取失败的情况，为了防止 CPU占用率过高，使用 sleep将协程阻塞一段时间
			time.Sleep(1 * time.Millisecond)
		}

		evm.StateDB.SubBalance(caller.Address(), value, tracing.BalanceChangeTransfer)


		//TODO:Gas计费
		if res.Err!=""{
			fmt.Println("跨片opcreate错误,",res.Err)
			evm.StateDB.RevertToSnapshot(snapshot)
			return res.Ret,res.Addr,0,errors.New(res.Err)
		}else{
			err = nil

			for _, item := range res.Journal {
				evm.StateDB.(*state.StateDB).GetJournal().Entries = append(evm.StateDB.(*state.StateDB).GetJournal().Entries, item.OriginalObj)
			}

			return res.Ret,address,gas,nil
		}





	}



}

// InitNewContract runs a new contract's creation code, performs checks on the
// resulting code that is to be deployed, and consumes necessary gas.
func (evm *EVM) InitNewContract(contract *Contract, address common.Address, value *uint256.Int) ([]byte, error) {
	// Charge the contract creation init gas in verkle mode
	//if evm.chainRules.IsEIP4762 {
	//	if !contract.UseGas(evm.AccessEvents.ContractCreateInitGas(address, value.Sign() != 0), evm.Config.Tracer, tracing.GasChangeWitnessContractInit) {
	//		//if !contract.UseGas(evm.AccessEvents.ContractCreateInitGas(address, value.Sign() != 0)) {
	//		return nil, ErrOutOfGas
	//	}
	//}

	ret, err := evm.interpreter.Run(contract, nil, false)
	if err != nil {
		return ret, err
	}

	// Check whether the max code size has been exceeded, assign err if the case.
	//if evm.chainRules.IsEIP158 && len(ret) > params.MaxCodeSize {
	//	return ret, ErrMaxCodeSizeExceeded
	//}

	// Reject code starting with 0xEF if EIP-3541 is enabled.
	//if len(ret) >= 1 && ret[0] == 0xEF && evm.chainRules.IsLondon {
	//	return ret, ErrInvalidCode
	//}

	//if !evm.chainRules.IsEIP4762 {
	//	createDataGas := uint64(len(ret)) * params.CreateDataGas
	//	if !contract.UseGas(createDataGas, evm.Config.Tracer, tracing.GasChangeCallCodeStorage) {
	//		//if !contract.UseGas(createDataGas) {
	//		return ret, ErrCodeStoreOutOfGas
	//	}
	//} else {
	//	// Contract creation completed, touch the missing fields in the contract
	//	if !contract.UseGas(evm.AccessEvents.AddAccount(address, true), evm.Config.Tracer, tracing.GasChangeWitnessContractCreation) {
	//		//if !contract.UseGas(evm.AccessEvents.AddAccount(address, true)) {
	//		return ret, ErrCodeStoreOutOfGas
	//	}
	//
	//	if len(ret) > 0 && !contract.UseGas(evm.AccessEvents.CodeChunksRangeGas(address, 0, uint64(len(ret)), uint64(len(ret)), true), evm.Config.Tracer, tracing.GasChangeWitnessCodeChunk) {
	//		//if len(ret) > 0 && !contract.UseGas(evm.AccessEvents.CodeChunksRangeGas(address, 0, uint64(len(ret)), uint64(len(ret)), true)) {
	//		return ret, ErrCodeStoreOutOfGas
	//	}
	//}

	evm.StateDB.SetCode(address, ret)
	return ret, nil
}
func BoolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
// Create creates a new contract using code as deployment code.
func (evm *EVM) Create(caller ContractRef, code []byte, gas uint64, value *uint256.Int,UUID string) (ret []byte, contractAddr common.Address, leftOverGas uint64, err error) {
	//新创建的合约地址与调用者地址和调用者的nonce有关
	//合约地址是可预测的，但需要等待上一个创建者账户中的 nonce 增加
	//适用于在合约之间直接通信，无需事先知道合约地址。
	//如果两个不同的创建者同时尝试使用相同的 nonce 创建合约，它们可能会发生 nonce 竞争，导致一个创建失败。
	contractAddr = crypto.CreateAddress(caller.Address(), evm.StateDB.GetNonce(caller.Address()))
	addr := hex.EncodeToString(contractAddr[:])
	i:= uint64(utils.Addr2Shard(addr)) != global.ShardID
	fmt.Println("要创建的合约地址是："+addr+" 是否是跨片创建："+BoolToString(i)," 目标片：",utils.Addr2Shard(addr))

	//if caller.Address() == common.HexToAddress("2000000000000000000000000000000000000001") {
	//	contractAddr = common.HexToAddress("1000000000000000000000000000000000000001")
	//}


	return evm.create(caller, &codeAndHash{code: code}, gas, value, contractAddr, CREATE,UUID)
}

// Create2 creates a new contract using code as deployment code.
//
// The different between Create2 with Create is Create2 uses keccak256(0xff ++ msg.sender ++ salt ++ keccak256(init_code))[12:]
// instead of the usual sender-and-nonce-hash as the address where the contract is initialized at.
func (evm *EVM) Create2(caller ContractRef, code []byte, gas uint64, endowment *uint256.Int, salt *uint256.Int,UUID string) (ret []byte, contractAddr common.Address, leftOverGas uint64, err error) {
	codeAndHash := &codeAndHash{code: code}
	//新创建的合约地址与调用者地址，salt 值和初始化代码 的 keccak256 哈希有关，可以通过选择不同的 salt 值来创建不同的地址
	//合约地址是在创建时就能够预测的，不受 nonce 的影响。
	//适用于在创建合约时预测合约地址，并通过地址存储信息，以便其他合约能够可靠地找到它。
	//使用不同的 salt，两个创建者可以同时创建具有相同初始化代码的合约，而不会发生地址冲突。
	contractAddr = crypto.CreateAddress2(caller.Address(), salt.Bytes32(), codeAndHash.Hash().Bytes())
	return evm.create(caller, codeAndHash, gas, endowment, contractAddr, CREATE2,UUID)
}

// ChainConfig returns the environment's chain configuration
func (evm *EVM) ChainConfig() *params.ChainConfig { return evm.chainConfig }

func (evm *EVM) captureBegin(depth int, typ OpCode, from common.Address, to common.Address, input []byte, startGas uint64, value *big.Int) {
	tracer := evm.Config.Tracer
	if tracer.OnEnter != nil {
		tracer.OnEnter(depth, byte(typ), from, to, input, startGas, value)
	}
	if tracer.OnGasChange != nil {
		tracer.OnGasChange(0, startGas, tracing.GasChangeCallInitialBalance)
	}
}

func (evm *EVM) captureEnd(depth int, startGas uint64, leftOverGas uint64, ret []byte, err error) {
	tracer := evm.Config.Tracer
	if leftOverGas != 0 && tracer.OnGasChange != nil {
		tracer.OnGasChange(leftOverGas, 0, tracing.GasChangeCallLeftOverReturned)
	}
	var reverted bool
	if err != nil {
		reverted = true
	}
	if !evm.chainRules.IsHomestead && errors.Is(err, ErrCodeStoreOutOfGas) {
		reverted = false
	}
	if tracer.OnExit != nil {
		tracer.OnExit(depth, ret, startGas-leftOverGas, VMErrorFromErr(err), reverted)
	}
}

// GetVMContext provides context about the block being executed as well as state
// to the tracers.
func (evm *EVM) GetVMContext() *tracing.VMContext {
	return &tracing.VMContext{
		Coinbase:    evm.Context.Coinbase,
		BlockNumber: evm.Context.BlockNumber,
		Time:        evm.Context.Time,
		Random:      evm.Context.Random,
		GasPrice:    evm.TxContext.GasPrice,
		ChainConfig: evm.ChainConfig(),
		StateDB:     evm.StateDB,
	}
}

// Call的备份
// Call executes the contract associated with the addr with the given input as
// parameters. It also handles any necessary value transfer required and takes
// the necessary steps to create accounts and reverses the state in case of an
// execution error or failed value transfer.
func (evm *EVM) CallBackup(caller ContractRef, addr common.Address, input []byte, gas uint64, value *uint256.Int) (ret []byte, leftOverGas uint64, err error) {
	// Capture the tracer start/end events in debug mode
	if evm.Config.Tracer != nil {
		evm.captureBegin(evm.depth, CALL, caller.Address(), addr, input, gas, value.ToBig())
		defer func(startGas uint64) {
			evm.captureEnd(evm.depth, startGas, leftOverGas, ret, err)
		}(gas)
	}
	// Fail if we're trying to execute above the call depth limit
	if evm.depth > int(params.CallCreateDepth) {
		return nil, gas, ErrDepth
	}
	// Fail if we're trying to transfer more than the available balance
	//staticcall、delegatecall无需检查
	//callcode 无需!value.IsZero() 需要!evm.Context.CanTransfer(evm.StateDB, caller.Address(), value)
	if !value.IsZero() && !evm.Context.CanTransfer(evm.StateDB, caller.Address(), value) {
		return nil, gas, ErrInsufficientBalance
	}
	snapshot := evm.StateDB.Snapshot()
	p, isPrecompile := evm.precompile(addr)

	if !evm.StateDB.Exist(addr) {
		//if !isPrecompile && evm.chainRules.IsEIP4762 {
		//	// add proof of absence to witness
		//	wgas := evm.AccessEvents.AddAccount(addr, false)
		//	if gas < wgas {
		//		evm.StateDB.RevertToSnapshot(snapshot)
		//		return nil, 0, ErrOutOfGas
		//	}
		//	gas -= wgas
		//}
		//
		//if !isPrecompile && evm.chainRules.IsEIP158 && value.IsZero() {
		//	// Calling a non-existing account, don't do anything.
		//	return nil, gas, nil
		//}
		evm.StateDB.CreateAccount(addr)
	}
	//delegatecall无需 callcode无需
	evm.Context.Transfer(evm.StateDB, caller.Address(), addr, value)

	if isPrecompile {
		ret, gas, err = RunPrecompiledContract(p, input, gas, evm.Config.Tracer)
	} else {
		// Initialise a new contract and set the code that is to be used by the EVM.
		// The contract is a scoped environment for this execution context only.
		code := evm.StateDB.GetCode(addr)
		if witness := evm.StateDB.Witness(); witness != nil {
			witness.AddCode(code)
		}
		if len(code) == 0 {
			ret, err = nil, nil // gas is unchanged
		} else {
			addrCopy := addr
			// If the account has no code, we can abort here
			// The depth-check is already done, and precompiles handled above
			//delegatecall为NewContract(caller, AccountRef(caller.Address()), nil, gas).AsDelegate()
			//callcode为NewContract(caller, AccountRef(caller.Address()), value, gas)
			//staticcall为NewContract(caller, AccountRef(addrCopy), new(uint256.Int), gas)
			contract := NewContract(caller, AccountRef(addrCopy), value, gas)
			contract.SetCallCode(&addrCopy, evm.StateDB.GetCodeHash(addrCopy), code)
			//staticcall的readonly为true
			ret, err = evm.interpreter.Run(contract, input, false)
			gas = contract.Gas
		}
	}
	// When an error was returned by the EVM or when setting the creation code
	// above we revert to the snapshot and consume any gas remaining. Additionally,
	// when we're in homestead this also counts for code storage gas errors.
	if err != nil {
		evm.StateDB.RevertToSnapshot(snapshot)
		if err != ErrExecutionReverted {
			if evm.Config.Tracer != nil && evm.Config.Tracer.OnGasChange != nil {
				evm.Config.Tracer.OnGasChange(gas, 0, tracing.GasChangeCallFailedExecution)
			}

			gas = 0
		}
		// TODO: consider clearing up unused snapshots:
		//} else {
		//	evm.StateDB.DiscardSnapshot(snapshot)
	}
	return ret, gas, err
}
