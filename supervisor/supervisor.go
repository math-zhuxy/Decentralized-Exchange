// Supervisor is an abstract role in this simulator that may read txs, generate partition infos,
// and handle history data.

package supervisor

import (
	"blockEmulator/client"
	"blockEmulator/core"
	"blockEmulator/global"
	"blockEmulator/message"
	"blockEmulator/networks"
	"blockEmulator/params"
	"blockEmulator/service"
	"blockEmulator/supervisor/committee"
	"blockEmulator/supervisor/measure"
	"blockEmulator/supervisor/signal"
	"blockEmulator/supervisor/supervisor_log"
	"blockEmulator/utils"
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Supervisor struct {
	// basic infos
	IPaddr       string // ip address of this Supervisor
	ChainConfig  *params.ChainConfig
	Ip_nodeTable map[uint64]map[uint64]string

	// tcp control
	listenStop bool
	tcpLn      net.Listener
	tcpLock    sync.Mutex
	// logger module
	sl *supervisor_log.SupervisorLog

	// control components
	Ss *signal.StopSignal // to control the stop message sending

	// supervisor and committee components
	ComMod committee.CommitteeModule

	// measure components
	testMeasureMods []measure.MeasureModule

	CommitteeMethod string
	// diy, add more structures or classes here ...

	contractCreationMap      map[uint64]string
	contractCreationMapMutex sync.Mutex

	contractAllowMap      map[uint64]map[string]struct{}
	contractAllowMapMutex sync.Mutex
}

func (d *Supervisor) NewSupervisor(ip string, pcc *params.ChainConfig, committeeMethod string, measureModNames ...string) {
	d.IPaddr = ip
	d.ChainConfig = pcc
	d.Ip_nodeTable = params.IPmap_nodeTable
	d.CommitteeMethod = committeeMethod
	d.sl = supervisor_log.NewSupervisorLog()

	d.Ss = signal.NewStopSignal(2 * int(pcc.ShardNums))
	d.contractCreationMap = make(map[uint64]string)
	d.contractAllowMap = make(map[uint64]map[string]struct{})
	for sid := uint64(0); sid < uint64(params.ShardNum); sid++ {
		d.contractAllowMap[sid] = make(map[string]struct{})
	}
	d.ComMod = committee.NewBrokerCommitteeMod_b2e(d.Ip_nodeTable, d.Ss, d.sl, params.FileInput, params.TotalDataSize, params.BatchSize)
	// switch committeeMethod {
	// case "CLPA_Broker":
	// 	d.ComMod = committee.NewCLPACommitteeMod_Broker(d.Ip_nodeTable, d.Ss, d.sl, params.FileInput, params.TotalDataSize, params.BatchSize, 80)
	// case "CLPA":
	// 	d.ComMod = committee.NewCLPACommitteeModule(d.Ip_nodeTable, d.Ss, d.sl, params.FileInput, params.TotalDataSize, params.BatchSize, 80)
	// case "Broker":
	// 	d.ComMod = committee.NewBrokerCommitteeMod(d.Ip_nodeTable, d.Ss, d.sl, params.FileInput, params.TotalDataSize, params.BatchSize)
	// case "Broker_b2e":
	// 	d.ComMod = committee.NewBrokerCommitteeMod_b2e(d.Ip_nodeTable, d.Ss, d.sl, params.FileInput, params.TotalDataSize, params.BatchSize)
	// default:
	// 	d.ComMod = committee.NewRelayCommitteeModule(d.Ip_nodeTable, d.Ss, d.sl, params.FileInput, params.TotalDataSize, params.BatchSize)
	// }

	d.testMeasureMods = make([]measure.MeasureModule, 0)
	for _, mModName := range measureModNames {
		switch mModName {
		case "TPS_Relay":
			d.testMeasureMods = append(d.testMeasureMods, measure.NewTestModule_avgTPS_Relay())
		case "TPS_Broker":
			d.testMeasureMods = append(d.testMeasureMods, measure.NewTestModule_avgTPS_Broker())
		case "TCL_Relay":
			d.testMeasureMods = append(d.testMeasureMods, measure.NewTestModule_TCL_Relay())
		case "TCL_Broker":
			d.testMeasureMods = append(d.testMeasureMods, measure.NewTestModule_TCL_Broker())
		case "CrossTxRate_Relay":
			d.testMeasureMods = append(d.testMeasureMods, measure.NewTestCrossTxRate_Relay())
		case "CrossTxRate_Broker":
			d.testMeasureMods = append(d.testMeasureMods, measure.NewTestCrossTxRate_Broker())
		case "TxNumberCount_Relay":
			d.testMeasureMods = append(d.testMeasureMods, measure.NewTestTxNumCount_Relay())
		case "TxNumberCount_Broker":
			d.testMeasureMods = append(d.testMeasureMods, measure.NewTestTxNumCount_Broker())
		default:
		}
	}
}

// Supervisor received the block information from the leaders, and handle these
// message to measure the performances.
func (d *Supervisor) handleBlockInfos(content []byte) {
	bim := new(message.BlockInfoMsg)
	err := json.Unmarshal(content, bim)
	if err != nil {
		log.Panic()
	}
	// StopSignal check
	if bim.BlockBodyLength == 0 {
		d.Ss.StopGap_Inc()
	} else {
		d.Ss.StopGap_Reset()
	}

	//now := time.Now()
	d.ComMod.HandleBlockInfo(bim)

	//fmt.Println("HandleBlockInfo耗时(ms)：", int(time.Since(now).Milliseconds()))

	// measure update
	//for _, measureMod := range d.testMeasureMods {
	//	measureMod.UpdateMeasureRecord(bim)
	//}
	// add codes here ...
}

// read transactions from dataFile. When the number of data is enough,
// the Supervisor will do re-partition and send partitionMSG and txs to leaders.

func (d *Supervisor) SupervisorTxHandling() {

	d.ComMod.MsgSendingControl()

	// TxHandling is end
	//for !d.Ss.GapEnough() { // wait all txs to be handled
	//	time.Sleep(time.Second)
	//}

	//add
	for true {
		time.Sleep(time.Second)
	}

	// send stop message
	stopmsg := message.MergeMessage(message.CStop, []byte("this is a stop message~"))
	d.sl.Slog.Println("Supervisor: now sending cstop message to all nodes")
	for sid := uint64(0); sid < d.ChainConfig.ShardNums; sid++ {
		for nid := uint64(0); nid < d.ChainConfig.Nodes_perShard; nid++ {
			networks.TcpDial(stopmsg, d.Ip_nodeTable[sid][nid])
		}
	}
	d.sl.Slog.Println("Supervisor: now Closing")
	d.listenStop = true
	d.CloseSupervisor()
}

const Broker2EarnAddr = "aaaa1a0ac6761fe4fcb6aca7dfa7e7c86e8dbc6d"
const TestSenderAddr = "aaaa1a0ac6761fe4fcb6aca7dfa7e7c800000000"
const TestSenderAddr2 = "aaaa1a0ac6761fe4fcb6aca7dfa7e7c800000001"

func (d *Supervisor) CreateBrokerContract() {
	contractFileName := "contract/broker.sol"
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("./contract/solc-windows.exe", contractFileName, "--abi", "--bin", "-o", "./")
	case "linux":
		cmd = exec.Command("./contract/solc-static-linux", contractFileName, "--abi", "--bin", "-o", "./")
	default:
		fmt.Printf("当前系统是 %s\n", runtime.GOOS)
		cmd = exec.Command("./contract/solc-static-linux", contractFileName, "--abi", "--bin", "-o", "./")
	}

	err := cmd.Run()
	if err != nil {
		log.Panic("执行命令出错:", err)
	}
	bytecode, err := readfromfile("./BrokerRegistry.bin")
	if err != nil {
		log.Panic(err)
	}

	createContractTransaction := core.NewTransactionContractBroker(Broker2EarnAddr, "", big.NewInt(0), 1, bytecode)
	createContractTransaction.GasPrice = big.NewInt(1)
	createContractTransaction.Gas = 500000
	createContractTransaction.UUID = uuid.New().String()

	var txs = []*core.Transaction{createContractTransaction}
	for sid := uint64(0); sid < uint64(params.ShardNum); sid++ {
		it := message.InjectTxs{
			Txs:       txs,
			ToShardID: sid,
		}
		itByte, err := json.Marshal(it)
		if err != nil {
			log.Panic(err)
		}
		send_msg := message.MergeMessage(message.CInject, itByte)
		go networks.TcpDial(send_msg, d.Ip_nodeTable[sid][0])
	}

	for true {
		d.contractCreationMapMutex.Lock()
		if len(d.contractCreationMap) == params.ShardNum {
			d.contractCreationMapMutex.Unlock()
			break
		}
		d.contractCreationMapMutex.Unlock()
		time.Sleep(time.Millisecond * 100)
	}

}

func (d *Supervisor) TestCrossShardContract4() {
	fmt.Println("测试开始...")
	contractFileName := "contract/test4/A.sol"
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("./contract/solc-windows.exe", contractFileName, "--abi", "--bin", "-o", "./bytecode")
	case "linux":
		cmd = exec.Command("./contract/solc-static-linux", contractFileName, "--abi", "--bin", "-o", "./bytecode")
	default:
		fmt.Printf("当前系统是 %s\n", runtime.GOOS)
		cmd = exec.Command("./contract/solc-static-linux", contractFileName, "--abi", "--bin", "-o", "./bytecode")
	}

	err := cmd.Run()
	if err != nil {
		log.Panic("执行命令出错:", err)
	}
	//得到合约A的字节码
	bytecode, err := readfromfile("./bytecode/A.bin")
	if err != nil {
		log.Panic(err)
	}

	//【测试1】部署合约A 涉及跨片create操作
	//创建部署合约A的交易，交易发起者地址TestSenderAddr与要创建的合约A的地址在不同分片，涉及跨片create操作
	fmt.Println("【测试1】部署合约A3")
	createContractTransaction := core.NewTransactionContract(TestSenderAddr2, "", big.NewInt(0), 1, bytecode)
	createContractTransaction.GasPrice = big.NewInt(1)
	createContractTransaction.Gas = 100000000
	createContractTransaction.UUID = uuid.New().String()
	var txs = []*core.Transaction{createContractTransaction}
	it := message.InjectTxs{
		Txs:       txs,
		ToShardID: uint64(utils.Addr2Shard(TestSenderAddr2)),
	}
	itByte, err := json.Marshal(it)
	if err != nil {
		log.Panic(err)
	}
	send_msg := message.MergeMessage(message.CInject, itByte)
	//发送部署合约A的交易
	go networks.TcpDial(send_msg, d.Ip_nodeTable[uint64(utils.Addr2Shard(TestSenderAddr2))][0])

	QueryContractResultResMap[createContractTransaction.UUID] = make(chan []byte)

	m := new(message.QueryContractResultReq)
	m.FromShardID = params.DeciderShard
	m.FromNodeID = 0
	m.UUID = createContractTransaction.UUID
	b, _ := json.Marshal(m)
	b1 := message.MergeMessage(message.CQueryContractResultReq, b)
	time.Sleep(time.Millisecond * 15000)
	//请求获取部署合约A这个交易的执行结果
	go networks.TcpDial(b1, d.Ip_nodeTable[uint64(utils.Addr2Shard(TestSenderAddr2))][0])

	//阻塞获取部署合约A这个交易的执行结果
	data, ok := <-QueryContractResultResMap[createContractTransaction.UUID]
	if !ok {
		// 通道已关闭
		fmt.Println("Channel closed")
		return
	}

	//成功部署了合约A, 获取了合约A的地址
	fmt.Println("合约地址为" + hex.EncodeToString(data))

	//【测试2】调用合约A的getA()函数
	fmt.Println("【测试2】调用合约A的getA()函数")
	s := "getA()"
	s1 := crypto.Keccak256Hash([]byte(s))
	fmt.Println(hex.EncodeToString(s1[:])[0:8])
	bb, _ := hex.DecodeString(hex.EncodeToString(s1[:])[0:8])

	//contractTransaction := core.NewTransactionContract(hex.EncodeToString(data), hex.EncodeToString(data), big.NewInt(0), 1, bb)
	contractTransaction := core.NewTransactionContract(hex.EncodeToString(data), hex.EncodeToString(data), big.NewInt(0), 1, bb)
	contractTransaction.GasPrice = big.NewInt(1)
	contractTransaction.Gas = 100000000
	contractTransaction.UUID = uuid.New().String()

	var txs1 = []*core.Transaction{contractTransaction}
	it1 := message.InjectTxs{
		Txs:       txs1,
		ToShardID: uint64(utils.Addr2Shard(hex.EncodeToString(data))),
	}
	itByte1, err := json.Marshal(it1)
	if err != nil {
		log.Panic(err)
	}
	send_msg1 := message.MergeMessage(message.CInject, itByte1)
	go networks.TcpDial(send_msg1, d.Ip_nodeTable[uint64(utils.Addr2Shard(hex.EncodeToString(data)))][0])

	QueryContractResultResMap[contractTransaction.UUID] = make(chan []byte)

	m1 := new(message.QueryContractResultReq)
	m1.FromShardID = params.DeciderShard
	m1.FromNodeID = 0
	m1.UUID = contractTransaction.UUID
	b11, _ := json.Marshal(m1)
	b111 := message.MergeMessage(message.CQueryContractResultReq, b11)
	time.Sleep(time.Millisecond * 20000)
	//请求获取合约A的test()函数返回的结果
	go networks.TcpDial(b111, d.Ip_nodeTable[uint64(utils.Addr2Shard(hex.EncodeToString(data)))][0])

	//阻塞获取合约A的test()函数返回的结果
	data1, ok := <-QueryContractResultResMap[contractTransaction.UUID]
	if !ok {
		// 通道已关闭
		fmt.Println("Channel closed")
		return
	}

	//
	fmt.Println("第一次调用getA()的执行结果为" + hex.EncodeToString(data1))

	//return

	//【测试3】调用合约A的testError()函数
	fmt.Println("【测试3】调用合约A的testError()函数")
	ss := "testError()"
	ss1 := crypto.Keccak256Hash([]byte(ss))
	fmt.Println(hex.EncodeToString(ss1[:])[0:8])
	bbb, _ := hex.DecodeString(hex.EncodeToString(ss1[:])[0:8])

	//contractTransaction := core.NewTransactionContract(hex.EncodeToString(data), hex.EncodeToString(data), big.NewInt(0), 1, bb)
	contractTransaction2 := core.NewTransactionContract(hex.EncodeToString(data), hex.EncodeToString(data), big.NewInt(0), 1, bbb)
	contractTransaction2.GasPrice = big.NewInt(1)
	contractTransaction2.Gas = 100000000
	contractTransaction2.UUID = uuid.New().String()

	var txs2 = []*core.Transaction{contractTransaction2}
	it2 := message.InjectTxs{
		Txs:       txs2,
		ToShardID: uint64(utils.Addr2Shard(hex.EncodeToString(data))),
	}
	itByte2, err := json.Marshal(it2)
	if err != nil {
		log.Panic(err)
	}
	send_msg2 := message.MergeMessage(message.CInject, itByte2)
	go networks.TcpDial(send_msg2, d.Ip_nodeTable[uint64(utils.Addr2Shard(hex.EncodeToString(data)))][0])

	QueryContractResultResMap[contractTransaction2.UUID] = make(chan []byte)

	m2 := new(message.QueryContractResultReq)
	m2.FromShardID = params.DeciderShard
	m2.FromNodeID = 0
	m2.UUID = contractTransaction2.UUID
	b12, _ := json.Marshal(m2)
	b112 := message.MergeMessage(message.CQueryContractResultReq, b12)
	time.Sleep(time.Millisecond * 20000)
	//请求获取合约A的testError()函数返回的结果
	go networks.TcpDial(b112, d.Ip_nodeTable[uint64(utils.Addr2Shard(hex.EncodeToString(data)))][0])

	//阻塞获取合约A的testError()函数返回的结果
	data2, ok := <-QueryContractResultResMap[contractTransaction2.UUID]
	if !ok {
		// 通道已关闭
		fmt.Println("Channel closed")
		return
	}

	//
	fmt.Println("调用testError()的执行结果为" + hex.EncodeToString(data2))

	//【测试4】调用合约A的getA()函数
	fmt.Println("【测试4】调用合约A的getA()函数")
	ss2 := "getA()"
	sss2 := crypto.Keccak256Hash([]byte(ss2))
	fmt.Println(hex.EncodeToString(sss2[:])[0:8])
	bbb2, _ := hex.DecodeString(hex.EncodeToString(sss2[:])[0:8])

	//contractTransaction := core.NewTransactionContract(hex.EncodeToString(data), hex.EncodeToString(data), big.NewInt(0), 1, bb)
	contractTransaction3 := core.NewTransactionContract(hex.EncodeToString(data), hex.EncodeToString(data), big.NewInt(0), 1, bbb2)
	contractTransaction3.GasPrice = big.NewInt(1)
	contractTransaction3.Gas = 100000000
	contractTransaction3.UUID = uuid.New().String()

	var txs3 = []*core.Transaction{contractTransaction3}
	it3 := message.InjectTxs{
		Txs:       txs3,
		ToShardID: uint64(utils.Addr2Shard(hex.EncodeToString(data))),
	}
	itByte3, err := json.Marshal(it3)
	if err != nil {
		log.Panic(err)
	}
	send_msg3 := message.MergeMessage(message.CInject, itByte3)
	go networks.TcpDial(send_msg3, d.Ip_nodeTable[uint64(utils.Addr2Shard(hex.EncodeToString(data)))][0])

	QueryContractResultResMap[contractTransaction3.UUID] = make(chan []byte)

	m3 := new(message.QueryContractResultReq)
	m3.FromShardID = params.DeciderShard
	m3.FromNodeID = 0
	m3.UUID = contractTransaction3.UUID
	b13, _ := json.Marshal(m3)
	b113 := message.MergeMessage(message.CQueryContractResultReq, b13)
	time.Sleep(time.Millisecond * 20000)
	//请求获取合约A的testError()函数返回的结果
	go networks.TcpDial(b113, d.Ip_nodeTable[uint64(utils.Addr2Shard(hex.EncodeToString(data)))][0])

	//阻塞获取合约A的testError()函数返回的结果
	data3, ok := <-QueryContractResultResMap[contractTransaction3.UUID]
	if !ok {
		// 通道已关闭
		fmt.Println("Channel closed")
		return
	}

	//
	fmt.Println("第二次调用getA()的执行结果为" + hex.EncodeToString(data3))

	fmt.Println("完成测试")

}

func (d *Supervisor) TestCrossShardContract3() {
	fmt.Println("测试开始...")
	contractFileName := "contract/test3/A2.sol"
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("./contract/solc-windows.exe", contractFileName, "--abi", "--bin", "-o", "./bytecode")
	case "linux":
		cmd = exec.Command("./contract/solc-static-linux", contractFileName, "--abi", "--bin", "-o", "./bytecode")
	default:
		fmt.Printf("当前系统是 %s\n", runtime.GOOS)
		cmd = exec.Command("./contract/solc-static-linux", contractFileName, "--abi", "--bin", "-o", "./bytecode")
	}

	err := cmd.Run()
	if err != nil {
		log.Panic("执行命令出错:", err)
	}
	//得到合约A的字节码
	bytecode, err := readfromfile("./bytecode/A2.bin")
	if err != nil {
		log.Panic(err)
	}

	//【测试1】部署合约A 涉及跨片create操作
	//创建部署合约A的交易，交易发起者地址TestSenderAddr与要创建的合约A的地址在不同分片，涉及跨片create操作
	fmt.Println("【测试1】部署合约A2")
	createContractTransaction := core.NewTransactionContract(TestSenderAddr2, "", big.NewInt(0), 1, bytecode)
	createContractTransaction.GasPrice = big.NewInt(1)
	createContractTransaction.Gas = 100000000
	createContractTransaction.UUID = uuid.New().String()
	var txs = []*core.Transaction{createContractTransaction}
	it := message.InjectTxs{
		Txs:       txs,
		ToShardID: uint64(utils.Addr2Shard(TestSenderAddr2)),
	}
	itByte, err := json.Marshal(it)
	if err != nil {
		log.Panic(err)
	}
	send_msg := message.MergeMessage(message.CInject, itByte)
	//发送部署合约A的交易
	go networks.TcpDial(send_msg, d.Ip_nodeTable[uint64(utils.Addr2Shard(TestSenderAddr2))][0])

	QueryContractResultResMap[createContractTransaction.UUID] = make(chan []byte)

	m := new(message.QueryContractResultReq)
	m.FromShardID = params.DeciderShard
	m.FromNodeID = 0
	m.UUID = createContractTransaction.UUID
	b, _ := json.Marshal(m)
	b1 := message.MergeMessage(message.CQueryContractResultReq, b)
	time.Sleep(time.Millisecond * 15000)
	//请求获取部署合约A这个交易的执行结果
	go networks.TcpDial(b1, d.Ip_nodeTable[uint64(utils.Addr2Shard(TestSenderAddr2))][0])

	//阻塞获取部署合约A这个交易的执行结果
	data, ok := <-QueryContractResultResMap[createContractTransaction.UUID]
	if !ok {
		// 通道已关闭
		fmt.Println("Channel closed")
		return
	}

	//成功部署了合约A, 获取了合约A的地址
	fmt.Println("合约地址为" + hex.EncodeToString(data))

	//【测试2】调用合约A的getA()函数
	fmt.Println("【测试2】调用合约A的getA()函数")
	s := "getA()"
	s1 := crypto.Keccak256Hash([]byte(s))
	fmt.Println(hex.EncodeToString(s1[:])[0:8])
	bb, _ := hex.DecodeString(hex.EncodeToString(s1[:])[0:8])

	//contractTransaction := core.NewTransactionContract(hex.EncodeToString(data), hex.EncodeToString(data), big.NewInt(0), 1, bb)
	contractTransaction := core.NewTransactionContract(hex.EncodeToString(data), hex.EncodeToString(data), big.NewInt(0), 1, bb)
	contractTransaction.GasPrice = big.NewInt(1)
	contractTransaction.Gas = 100000000
	contractTransaction.UUID = uuid.New().String()

	var txs1 = []*core.Transaction{contractTransaction}
	it1 := message.InjectTxs{
		Txs:       txs1,
		ToShardID: uint64(utils.Addr2Shard(hex.EncodeToString(data))),
	}
	itByte1, err := json.Marshal(it1)
	if err != nil {
		log.Panic(err)
	}
	send_msg1 := message.MergeMessage(message.CInject, itByte1)
	go networks.TcpDial(send_msg1, d.Ip_nodeTable[uint64(utils.Addr2Shard(hex.EncodeToString(data)))][0])

	QueryContractResultResMap[contractTransaction.UUID] = make(chan []byte)

	m1 := new(message.QueryContractResultReq)
	m1.FromShardID = params.DeciderShard
	m1.FromNodeID = 0
	m1.UUID = contractTransaction.UUID
	b11, _ := json.Marshal(m1)
	b111 := message.MergeMessage(message.CQueryContractResultReq, b11)
	time.Sleep(time.Millisecond * 20000)
	//请求获取合约A的test()函数返回的结果
	go networks.TcpDial(b111, d.Ip_nodeTable[uint64(utils.Addr2Shard(hex.EncodeToString(data)))][0])

	//阻塞获取合约A的test()函数返回的结果
	data1, ok := <-QueryContractResultResMap[contractTransaction.UUID]
	if !ok {
		// 通道已关闭
		fmt.Println("Channel closed")
		return
	}

	//
	fmt.Println("第一次调用getA()的执行结果为" + hex.EncodeToString(data1))

	//return

	//【测试3】调用合约A的testError()函数
	fmt.Println("【测试3】调用合约A的testError()函数")
	ss := "testError()"
	ss1 := crypto.Keccak256Hash([]byte(ss))
	fmt.Println(hex.EncodeToString(ss1[:])[0:8])
	bbb, _ := hex.DecodeString(hex.EncodeToString(ss1[:])[0:8])

	//contractTransaction := core.NewTransactionContract(hex.EncodeToString(data), hex.EncodeToString(data), big.NewInt(0), 1, bb)
	contractTransaction2 := core.NewTransactionContract(hex.EncodeToString(data), hex.EncodeToString(data), big.NewInt(0), 1, bbb)
	contractTransaction2.GasPrice = big.NewInt(1)
	contractTransaction2.Gas = 100000000
	contractTransaction2.UUID = uuid.New().String()

	var txs2 = []*core.Transaction{contractTransaction2}
	it2 := message.InjectTxs{
		Txs:       txs2,
		ToShardID: uint64(utils.Addr2Shard(hex.EncodeToString(data))),
	}
	itByte2, err := json.Marshal(it2)
	if err != nil {
		log.Panic(err)
	}
	send_msg2 := message.MergeMessage(message.CInject, itByte2)
	go networks.TcpDial(send_msg2, d.Ip_nodeTable[uint64(utils.Addr2Shard(hex.EncodeToString(data)))][0])

	QueryContractResultResMap[contractTransaction2.UUID] = make(chan []byte)

	m2 := new(message.QueryContractResultReq)
	m2.FromShardID = params.DeciderShard
	m2.FromNodeID = 0
	m2.UUID = contractTransaction2.UUID
	b12, _ := json.Marshal(m2)
	b112 := message.MergeMessage(message.CQueryContractResultReq, b12)
	time.Sleep(time.Millisecond * 20000)
	//请求获取合约A的testError()函数返回的结果
	go networks.TcpDial(b112, d.Ip_nodeTable[uint64(utils.Addr2Shard(hex.EncodeToString(data)))][0])

	//阻塞获取合约A的testError()函数返回的结果
	data2, ok := <-QueryContractResultResMap[contractTransaction2.UUID]
	if !ok {
		// 通道已关闭
		fmt.Println("Channel closed")
		return
	}

	//
	fmt.Println("调用testError()的执行结果为" + hex.EncodeToString(data2))

	//【测试4】调用合约A的getA()函数
	fmt.Println("【测试4】调用合约A的getA()函数")
	ss2 := "getA()"
	sss2 := crypto.Keccak256Hash([]byte(ss2))
	fmt.Println(hex.EncodeToString(sss2[:])[0:8])
	bbb2, _ := hex.DecodeString(hex.EncodeToString(sss2[:])[0:8])

	//contractTransaction := core.NewTransactionContract(hex.EncodeToString(data), hex.EncodeToString(data), big.NewInt(0), 1, bb)
	contractTransaction3 := core.NewTransactionContract(hex.EncodeToString(data), hex.EncodeToString(data), big.NewInt(0), 1, bbb2)
	contractTransaction3.GasPrice = big.NewInt(1)
	contractTransaction3.Gas = 100000000
	contractTransaction3.UUID = uuid.New().String()

	var txs3 = []*core.Transaction{contractTransaction3}
	it3 := message.InjectTxs{
		Txs:       txs3,
		ToShardID: uint64(utils.Addr2Shard(hex.EncodeToString(data))),
	}
	itByte3, err := json.Marshal(it3)
	if err != nil {
		log.Panic(err)
	}
	send_msg3 := message.MergeMessage(message.CInject, itByte3)
	go networks.TcpDial(send_msg3, d.Ip_nodeTable[uint64(utils.Addr2Shard(hex.EncodeToString(data)))][0])

	QueryContractResultResMap[contractTransaction3.UUID] = make(chan []byte)

	m3 := new(message.QueryContractResultReq)
	m3.FromShardID = params.DeciderShard
	m3.FromNodeID = 0
	m3.UUID = contractTransaction3.UUID
	b13, _ := json.Marshal(m3)
	b113 := message.MergeMessage(message.CQueryContractResultReq, b13)
	time.Sleep(time.Millisecond * 20000)
	//请求获取合约A的testError()函数返回的结果
	go networks.TcpDial(b113, d.Ip_nodeTable[uint64(utils.Addr2Shard(hex.EncodeToString(data)))][0])

	//阻塞获取合约A的testError()函数返回的结果
	data3, ok := <-QueryContractResultResMap[contractTransaction3.UUID]
	if !ok {
		// 通道已关闭
		fmt.Println("Channel closed")
		return
	}

	//
	fmt.Println("第二次调用getA()的执行结果为" + hex.EncodeToString(data3))

	fmt.Println("完成测试")

}

func (d *Supervisor) TestCrossShardContract2() {
	fmt.Println("测试开始...")
	contractFileName := "contract/test2/A1.sol"
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("./contract/solc-windows.exe", contractFileName, "--abi", "--bin", "-o", "./bytecode")
	case "linux":
		cmd = exec.Command("./contract/solc-static-linux", contractFileName, "--abi", "--bin", "-o", "./bytecode")
	default:
		fmt.Printf("当前系统是 %s\n", runtime.GOOS)
		cmd = exec.Command("./contract/solc-static-linux", contractFileName, "--abi", "--bin", "-o", "./bytecode")
	}

	err := cmd.Run()
	if err != nil {
		log.Panic("执行命令出错:", err)
	}
	//得到合约A的字节码
	bytecode, err := readfromfile("./bytecode/A1.bin")
	if err != nil {
		log.Panic(err)
	}

	//【测试1】部署合约A 涉及跨片create操作
	//创建部署合约A的交易，交易发起者地址TestSenderAddr与要创建的合约A的地址在不同分片，涉及跨片create操作
	createContractTransaction := core.NewTransactionContract(TestSenderAddr, "", big.NewInt(0), 1, bytecode)
	createContractTransaction.GasPrice = big.NewInt(1)
	createContractTransaction.Gas = 100000000
	createContractTransaction.UUID = uuid.New().String()
	var txs = []*core.Transaction{createContractTransaction}
	it := message.InjectTxs{
		Txs:       txs,
		ToShardID: uint64(utils.Addr2Shard(TestSenderAddr)),
	}
	itByte, err := json.Marshal(it)
	if err != nil {
		log.Panic(err)
	}
	send_msg := message.MergeMessage(message.CInject, itByte)
	//发送部署合约A的交易
	go networks.TcpDial(send_msg, d.Ip_nodeTable[uint64(utils.Addr2Shard(TestSenderAddr))][0])

	QueryContractResultResMap[createContractTransaction.UUID] = make(chan []byte)

	m := new(message.QueryContractResultReq)
	m.FromShardID = params.DeciderShard
	m.FromNodeID = 0
	m.UUID = createContractTransaction.UUID
	b, _ := json.Marshal(m)
	b1 := message.MergeMessage(message.CQueryContractResultReq, b)
	time.Sleep(time.Millisecond * 15000)
	//请求获取部署合约A这个交易的执行结果
	go networks.TcpDial(b1, d.Ip_nodeTable[uint64(utils.Addr2Shard(TestSenderAddr))][0])

	//阻塞获取部署合约A这个交易的执行结果
	data, ok := <-QueryContractResultResMap[createContractTransaction.UUID]
	if !ok {
		// 通道已关闭
		fmt.Println("Channel closed")
		return
	}

	//成功部署了合约A, 获取了合约A的地址
	fmt.Println("合约地址为" + hex.EncodeToString(data))

	//【测试2】调用合约A的create()函数，合约A执行create()函数时，会跨片创建合约B，并调用合约B的getA(uint256)函数.因此涉及了跨片create、跨片call和片内读写这些操作
	s := "test()"
	s1 := crypto.Keccak256Hash([]byte(s))
	fmt.Println(hex.EncodeToString(s1[:])[0:8])
	bb, _ := hex.DecodeString(hex.EncodeToString(s1[:])[0:8])

	//contractTransaction := core.NewTransactionContract(hex.EncodeToString(data), hex.EncodeToString(data), big.NewInt(0), 1, bb)
	contractTransaction := core.NewTransactionContract(hex.EncodeToString(data), hex.EncodeToString(data), big.NewInt(0), 1, bb)
	contractTransaction.GasPrice = big.NewInt(1)
	contractTransaction.Gas = 100000000
	contractTransaction.UUID = uuid.New().String()

	var txs1 = []*core.Transaction{contractTransaction}
	it1 := message.InjectTxs{
		Txs:       txs1,
		ToShardID: uint64(utils.Addr2Shard(hex.EncodeToString(data))),
	}
	itByte1, err := json.Marshal(it1)
	if err != nil {
		log.Panic(err)
	}
	send_msg1 := message.MergeMessage(message.CInject, itByte1)
	go networks.TcpDial(send_msg1, d.Ip_nodeTable[uint64(utils.Addr2Shard(hex.EncodeToString(data)))][0])

	QueryContractResultResMap[contractTransaction.UUID] = make(chan []byte)

	m1 := new(message.QueryContractResultReq)
	m1.FromShardID = params.DeciderShard
	m1.FromNodeID = 0
	m1.UUID = contractTransaction.UUID
	b11, _ := json.Marshal(m1)
	b111 := message.MergeMessage(message.CQueryContractResultReq, b11)
	time.Sleep(time.Millisecond * 20000)
	//请求获取合约A的test()函数返回的结果
	go networks.TcpDial(b111, d.Ip_nodeTable[uint64(utils.Addr2Shard(hex.EncodeToString(data)))][0])

	//阻塞获取合约A的test()函数返回的结果
	data1, ok := <-QueryContractResultResMap[contractTransaction.UUID]
	if !ok {
		// 通道已关闭
		fmt.Println("Channel closed")
		return
	}

	//返回结果是 ,是十进制40的16进制的256位扩展表示，符合预期
	fmt.Println("调用test()的执行结果为" + hex.EncodeToString(data1))

	fmt.Println("完成测试")

}

// 测试跨片合约的调用
func (d *Supervisor) TestCrossShardContract1() {
	fmt.Println("测试开始...")
	//编译合约A，合约A导入了合约B，因此B也会被一起编译
	contractFileName := "contract/test1/A.sol"
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("./contract/solc-windows.exe", contractFileName, "--abi", "--bin", "-o", "./bytecode")
	case "linux":
		cmd = exec.Command("./contract/solc-static-linux", contractFileName, "--abi", "--bin", "-o", "./bytecode")
	default:
		fmt.Printf("当前系统是 %s\n", runtime.GOOS)
		cmd = exec.Command("./contract/solc-static-linux", contractFileName, "--abi", "--bin", "-o", "./bytecode")
	}

	err := cmd.Run()
	if err != nil {
		log.Panic("执行命令出错:", err)
	}
	//得到合约A的字节码
	bytecode, err := readfromfile("./bytecode/A.bin")
	if err != nil {
		log.Panic(err)
	}

	//【测试1】部署合约A 涉及跨片create操作
	//创建部署合约A的交易，交易发起者地址TestSenderAddr与要创建的合约A的地址在不同分片，涉及跨片create操作
	createContractTransaction := core.NewTransactionContract(TestSenderAddr, "", big.NewInt(0), 1, bytecode)
	createContractTransaction.GasPrice = big.NewInt(1)
	createContractTransaction.Gas = 1000000
	createContractTransaction.UUID = uuid.New().String()
	var txs = []*core.Transaction{createContractTransaction}
	it := message.InjectTxs{
		Txs:       txs,
		ToShardID: uint64(utils.Addr2Shard(TestSenderAddr)),
	}
	itByte, err := json.Marshal(it)
	if err != nil {
		log.Panic(err)
	}
	send_msg := message.MergeMessage(message.CInject, itByte)
	//发送部署合约A的交易
	go networks.TcpDial(send_msg, d.Ip_nodeTable[uint64(utils.Addr2Shard(TestSenderAddr))][0])

	QueryContractResultResMap[createContractTransaction.UUID] = make(chan []byte)

	m := new(message.QueryContractResultReq)
	m.FromShardID = params.DeciderShard
	m.FromNodeID = 0
	m.UUID = createContractTransaction.UUID
	b, _ := json.Marshal(m)
	b1 := message.MergeMessage(message.CQueryContractResultReq, b)
	time.Sleep(time.Millisecond * 15000)
	//请求获取部署合约A这个交易的执行结果
	go networks.TcpDial(b1, d.Ip_nodeTable[uint64(utils.Addr2Shard(TestSenderAddr))][0])

	//阻塞获取部署合约A这个交易的执行结果
	data, ok := <-QueryContractResultResMap[createContractTransaction.UUID]
	if !ok {
		// 通道已关闭
		fmt.Println("Channel closed")
		return
	}

	//成功部署了合约A, 获取了合约A的地址
	fmt.Println("合约地址为" + hex.EncodeToString(data))

	//【测试2】调用合约A的create()函数，合约A执行create()函数时，会跨片创建合约B，并调用合约B的getA(uint256)函数.因此涉及了跨片create、跨片call和片内读写这些操作
	s := "create()"
	s1 := crypto.Keccak256Hash([]byte(s))
	fmt.Println(hex.EncodeToString(s1[:])[0:8])
	bb, _ := hex.DecodeString(hex.EncodeToString(s1[:])[0:8])

	//contractTransaction := core.NewTransactionContract(hex.EncodeToString(data), hex.EncodeToString(data), big.NewInt(0), 1, bb)
	contractTransaction := core.NewTransactionContract(hex.EncodeToString(data), hex.EncodeToString(data), big.NewInt(0), 1, bb)
	contractTransaction.GasPrice = big.NewInt(1)
	contractTransaction.Gas = 10000000
	contractTransaction.UUID = uuid.New().String()

	var txs1 = []*core.Transaction{contractTransaction}
	it1 := message.InjectTxs{
		Txs:       txs1,
		ToShardID: uint64(utils.Addr2Shard(hex.EncodeToString(data))),
	}
	itByte1, err := json.Marshal(it1)
	if err != nil {
		log.Panic(err)
	}
	send_msg1 := message.MergeMessage(message.CInject, itByte1)
	go networks.TcpDial(send_msg1, d.Ip_nodeTable[uint64(utils.Addr2Shard(hex.EncodeToString(data)))][0])

	QueryContractResultResMap[contractTransaction.UUID] = make(chan []byte)

	m1 := new(message.QueryContractResultReq)
	m1.FromShardID = params.DeciderShard
	m1.FromNodeID = 0
	m1.UUID = contractTransaction.UUID
	b11, _ := json.Marshal(m1)
	b111 := message.MergeMessage(message.CQueryContractResultReq, b11)
	time.Sleep(time.Millisecond * 20000)
	//请求获取合约A的create()函数返回的结果
	go networks.TcpDial(b111, d.Ip_nodeTable[uint64(utils.Addr2Shard(hex.EncodeToString(data)))][0])

	//阻塞获取合约A的create()函数返回的结果
	data1, ok := <-QueryContractResultResMap[contractTransaction.UUID]
	if !ok {
		// 通道已关闭
		fmt.Println("Channel closed")
		return
	}

	//返回结果是0000000000000000000000000000000000000000000000000000000000000014,是十进制20的16进制的256位扩展表示，符合预期
	fmt.Println("调用create()的执行结果为" + hex.EncodeToString(data1))

	fmt.Println("完成测试")

}
func readfromfile(filename string) ([]byte, error) {

	data, err := os.ReadFile(filename)
	if err != nil {
		fmt.Println("Error reading file:", err)
		return nil, err
	}

	strData := string(data)
	fmt.Println("File content as string:", strData)

	byteData, err := hex.DecodeString((strData))
	if err != nil {
		log.Panic(err)
	}
	return byteData, err
}

type ParamsElement map[string]interface{}

// 定义一个请求体结构，匹配你的JSON RPC请求
type RpcRequest struct {
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	Id      interface{}   `json:"id"`
	Jsonrpc string        `json:"jsonrpc"`
}

type RpcResponse struct {
	Jsonrpc string      `json:"jsonrpc"`
	Id      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}
type BlockInfo struct {
	Number           string        `json:"number"`
	Hash             string        `json:"hash"`
	ParentHash       string        `json:"parentHash"`
	Nonce            string        `json:"nonce,omitempty"`
	Sha3Uncles       string        `json:"sha3Uncles"`
	LogsBloom        string        `json:"logsBloom,omitempty"`
	TransactionsRoot string        `json:"transactionsRoot"`
	StateRoot        string        `json:"stateRoot"`
	ReceiptsRoot     string        `json:"receiptsRoot"`
	Miner            string        `json:"miner"`
	Difficulty       string        `json:"difficulty"`
	TotalDifficulty  string        `json:"totalDifficulty"`
	ExtraData        string        `json:"extraData"`
	Size             string        `json:"size"`
	GasLimit         string        `json:"gasLimit"`
	GasUsed          string        `json:"gasUsed"`
	Timestamp        string        `json:"timestamp"`
	Transactions     []interface{} `json:"transactions"`
	Uncles           []string      `json:"uncles"`
}

type TransactionResult struct {
	BlockHash        string `json:"blockHash"`
	BlockNumber      string `json:"blockNumber"`
	From             string `json:"from"`
	Gas              string `json:"gas"`
	GasPrice         string `json:"gasPrice"`
	Hash             string `json:"hash"`
	Input            string `json:"input"`
	Nonce            string `json:"nonce"`
	To               string `json:"to"`
	TransactionIndex string `json:"transactionIndex"`
	Value            string `json:"value"`
	V                string `json:"v"` // ECDSA签名的组成部分
	R                string `json:"r"`
	S                string `json:"s"`
}

type TxRec struct {
	TransactionHash string `json:"transactionHash"`
	ContractAddress string `json:"contractAddress"`
	GasUsed         string `json:"gasUsed"`
	Status          string `json:"status"`
	//Logs []LogItem `json:"logs"`
}

type LogItem struct {
	Data string `json:"data"`
}

var UUIDToFromMap = make(map[string]string)
var blockNumber = 0

func (d *Supervisor) RunHTTP() error {

	go func() {
		for true {
			blockNumber++
			time.Sleep(time.Millisecond * 1000)
		}
	}()
	var Clt = client.NewClient("127.0.0.1:17777")

	r := gin.Default()
	r.Use(CorsConfig())
	r.Use(func(c *gin.Context) {
		// 将 Supervisor 实例存储在上下文中
		c.Set("supervisor", d)
		c.Set("clt", Clt)
		c.Next()
	})

	go func() {
		if err := Clt.ClientListen(); err != nil {
			log.Fatalf("Failed to start client listener: %v", err)
		}
	}()
	fmt.Println("Now client begin listening.")

	r.POST("/", func(c *gin.Context) {
		d := c.MustGet("supervisor").(*Supervisor)
		var request RpcRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, RpcResponse{
				Jsonrpc: "2.0",
				Id:      request.Id,
				Error:   map[string]interface{}{"code": -32700, "message": "Parse error"},
			})
			return
		}

		// 根据method字段的值来处理不同的请求
		response := RpcResponse{
			Jsonrpc: "2.0",
			Id:      request.Id,
		}

		switch request.Method {
		case "eth_getBlockByNumber":
			b := &BlockInfo{
				Number:           "0x1b4",
				Hash:             "0xdc0818cf78f21a8e70579cb46a43643f78291264dda342ae31049421c82d21ae",
				ParentHash:       "",
				Nonce:            "",
				Sha3Uncles:       "",
				LogsBloom:        "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
				TransactionsRoot: "",
				StateRoot:        "",
				ReceiptsRoot:     "",
				Miner:            "0xbb7b8287f3f0a933474a79eae42cbca977791171",
				Difficulty:       "",
				TotalDifficulty:  "",
				ExtraData:        "",
				Size:             "",
				GasLimit:         "",
				GasUsed:          "",
				Timestamp:        "",
				Transactions:     nil,
				Uncles:           nil,
			}
			response.Result = b
		case "eth_chainId":
			response.Result = "0x1"
		case "net_version":
			response.Result = "1"
		case "eth_accounts":
			response.Result = []string{
				"0x0000000000000000000000000000000000000000",
				"0x117d73d8a49eeb85d32cf465507dd71d507100c2",
				"0x127d73d8a49eeb85d32cf465507dd71d507100c3",
				"0x137d73d8a49eeb85d32cf465507dd71d507100c4",
				"0x147d73d8a49eeb85d32cf465507dd71d507100c5",
				"0x157d73d8a49eeb85d32cf465507dd71d507100c6",
				"0x167d73d8a49eeb85d32cf465507dd71d507100c7",
				"0x177d73d8a49eeb85d32cf465507dd71d507100c8",
				"0x187d73d8a49eeb85d32cf465507dd71d507100c9",
				"0x197d73d8a49eeb85d32cf465507dd71d507100d0",
			}

		case "eth_getBalance":
			acc := request.Params[0].(string)[2:]
			m := new(message.QueryAccount)
			m.FromNodeID = 0
			m.FromShardID = params.DeciderShard
			m.Account = acc

			global.AccBalanceMapLock.Lock()
			delete(global.AccBalanceMap, acc)
			global.AccBalanceMapLock.Unlock()
			b, _ := json.Marshal(m)
			ms := message.MergeMessage(message.CQueryAccount, b)
			go networks.TcpDial(ms, d.Ip_nodeTable[uint64(utils.Addr2Shard(acc))][0])
			var Balance uint64
			for true {
				global.AccBalanceMapLock.Lock()
				if balance, ok := global.AccBalanceMap[acc]; ok {
					Balance = balance
					delete(global.AccBalanceMap, acc)
					global.AccBalanceMapLock.Unlock()
					break
				} else {
					global.AccBalanceMapLock.Unlock()
					time.Sleep(time.Millisecond * 100)
				}
			}

			n := big.NewInt(int64(Balance))
			wei, _ := big.NewInt(0).SetString("1000000000000000000", 10)
			n.Mul(n, wei)

			response.Result = "0x" + n.Text(16)

		case "eth_estimateGas":
			response.Result = "0x1"
		case "eth_blockNumber":
			response.Result = "0x" + fmt.Sprintf("%x", blockNumber)
		// 假设params[0]是一个字符串（如"latest"），params[1]是一个布尔值
		//if len(request.Params) == 2 {
		//	blockNumber, ok := request.Params[0].(string) // 这里假设是一个带有"hex"键的map，实际取决于你的具体需求
		//	includeTransactions, ok2 := request.Params[1].(bool)
		//	if ok && ok2 {
		//		// 处理逻辑...
		//		response.Result = fmt.Sprintf("Block number: %s, Include transactions: %t", blockNumber, includeTransactions)
		//	} else {
		//		response.Error = map[string]interface{}{"code": -32602, "message": "Invalid params"}
		//	}
		//} else {
		//	response.Error = map[string]interface{}{"code": -32602, "message": "Invalid params"}
		//}

		case "eth_sendTransaction":
			obj := request.Params[0].(map[string]interface{})
			from := obj["from"].(string)[2:]
			to := ""
			if obj["to"] != nil {
				to = obj["to"].(string)[2:]
			}
			//gas:=""
			//if obj["gas"] !=nil {
			//	gas = obj["gas"].(string)
			//}
			value := ""
			if obj["value"] != nil {
				value = obj["value"].(string)
			}
			input := ""
			if obj["data"] != nil {
				input = obj["data"].(string)
			} else if obj["input"] != nil {
				input = obj["input"].(string)
			}
			//gasPrice:=""
			//if obj["gasPrice"] != nil {
			//	gasPrice=obj["gasPrice"].(string)
			//}
			s := value[2:]
			if len(s)%2 != 0 {
				s = "0" + s
			}
			byteSlice, err := hex.DecodeString(s)
			var bigInt big.Int
			bigInt.SetBytes(byteSlice)

			inp, _ := hex.DecodeString(input[2:])
			fmt.Println("value为", bigInt.Uint64())
			fmt.Println("from:", from, ",to:", to)
			//fmt.Println("utils.Addr2Shard(from):",strconv.Itoa(utils.Addr2Shard(from)),",utils.Addr2Shard(to):",strconv.Itoa(utils.Addr2Shard(to)))
			fmt.Println("params.ShardNum:", strconv.Itoa(params.ShardNum))
			fmt.Println("params.ShardNum:", params.ShardNum)
			Transaction := core.NewTransactionContract(from, to, &bigInt, 1, inp)
			Transaction.GasPrice = big.NewInt(1)
			Transaction.Gas = 100000000
			randomBytes := make([]byte, 32)
			_, _ = rand.Read(randomBytes)
			hexString := hex.EncodeToString(randomBytes)
			Transaction.UUID = hexString
			UUIDToFromMap[hexString] = from
			var txs = []*core.Transaction{Transaction}
			it := message.InjectTxs{
				Txs:       txs,
				ToShardID: uint64(utils.Addr2Shard(from)),
			}
			itByte, err := json.Marshal(it)
			if err != nil {
				log.Panic(err)
			}
			send_msg := message.MergeMessage(message.CInject, itByte)
			go networks.TcpDial(send_msg, d.Ip_nodeTable[uint64(utils.Addr2Shard(from))][0])

			response.Result = "0x" + hexString
		case "eth_getTransactionReceipt":
			UUID := request.Params[0].(string)[2:]
			//QueryContractResultResMap[UUID] = make(chan []byte)
			//
			//m := new(message.QueryContractResultReq)
			//m.FromShardID = params.DeciderShard
			//m.FromNodeID = 0
			//m.UUID = UUID
			//b, _ := json.Marshal(m)
			//b1 := message.MergeMessage(message.CQueryContractResultReq, b)
			//go networks.TcpDial(b1, d.Ip_nodeTable[uint64(utils.Addr2Shard(UUIDToFromMap[UUID]))][0])
			//
			//data, ok := <-QueryContractResultResMap[UUID]
			//if !ok {
			//	// 通道已关闭
			//	fmt.Println("Channel closed")
			//	return
			//}
			//
			//fmt.Println("合约地址为" + hex.EncodeToString(data))

			now := time.Now()
			var tx message.TxInfo
			var ok bool
			for time.Since(now).Seconds() < 10 {
				TxInfoMapLock.Lock()
				tx, ok = TxInfoMap[UUID]
				if ok {
					TxInfoMapLock.Unlock()
					break
				}
				TxInfoMapLock.Unlock()
				time.Sleep(time.Millisecond * 100)
			}
			//res := "0x"+hex.EncodeToString(tx.Res)

			msg := &TxRec{
				TransactionHash: "0x" + UUID,
				ContractAddress: "0x" + hex.EncodeToString(tx.Res),
				GasUsed:         "0x1",
				Status:          "0x1",
			}

			if !tx.IsSuccess {
				msg.Status = "0x0"
			}
			//msg.Logs = make([]LogItem, 1)
			//msg.Logs[0] = LogItem{Data: res}
			response.Result = msg

		case "eth_getTransactionByHash":
			UUID := request.Params[0].(string)[2:]
			now := time.Now()
			var tx message.TxInfo
			var ok bool
			for time.Since(now).Seconds() < 10 {
				TxInfoMapLock.Lock()
				tx, ok = TxInfoMap[UUID]
				if ok {
					TxInfoMapLock.Unlock()
					break
				}
				TxInfoMapLock.Unlock()
				time.Sleep(time.Millisecond * 100)
			}

			r1 := &TransactionResult{
				BlockHash:        "0x1d59ff54b1eb26b013ce3cb5fc9dab3705b415a67127a003c3e61eb445bb8df2",
				BlockNumber:      "0x" + fmt.Sprintf("%x", blockNumber),
				From:             "0x" + hex.EncodeToString(tx.From[:]),
				Gas:              "0x" + fmt.Sprintf("%x", tx.GasLimit),
				GasPrice:         "0x" + fmt.Sprintf("%x", tx.GasPrice),
				Hash:             "0x" + UUID,
				Input:            "0x" + hex.EncodeToString(tx.Input[:]),
				Nonce:            "0x15",
				To:               "",
				TransactionIndex: "0x41",
				Value:            "0x" + tx.Value.Text(16),
				V:                "0x25",
				R:                "0x1b5e176d927f8e9ab405058b2d2457392da3e20f328b16ddabcebc33eaac5fea",
				S:                "0x4ba69724e8f69de52f0125ad8b3c5c2cef33019bac3249e2c0a2192766d1721c",
			}
			cc := common.Address{}
			if tx.To != cc {
				r1.To = "0x" + hex.EncodeToString(tx.To[:])
			}
			response.Result = r1

		case "eth_call":
			obj := request.Params[0].(map[string]interface{})
			from := obj["from"].(string)[2:]
			to := ""
			if obj["to"] != nil {
				to = obj["to"].(string)[2:]
			}
			input := ""
			if obj["data"] != nil {
				input = obj["data"].(string)
			} else if obj["input"] != nil {
				input = obj["input"].(string)
			}
			//gasPrice:=""
			//if obj["gasPrice"] != nil {
			//	gasPrice=obj["gasPrice"].(string)
			//}
			//s:=value[2:]
			//if len(s) %2!=0{
			//	s="0"+s
			//}
			//byteSlice, err := hex.DecodeString(s)
			//var bigInt big.Int
			//bigInt.SetBytes(byteSlice)

			inp, _ := hex.DecodeString(input[2:])

			Transaction := core.NewTransactionContract(from, to, big.NewInt(0), 1, inp)
			Transaction.GasPrice = big.NewInt(1)
			Transaction.Gas = 100000000
			randomBytes := make([]byte, 32)
			_, _ = rand.Read(randomBytes)
			hexString := hex.EncodeToString(randomBytes)
			Transaction.UUID = hexString
			UUIDToFromMap[hexString] = from
			var txs = []*core.Transaction{Transaction}
			it := message.InjectTxs{
				Txs:       txs,
				ToShardID: uint64(utils.Addr2Shard(from)),
			}
			itByte, err := json.Marshal(it)
			if err != nil {
				log.Panic(err)
			}
			send_msg := message.MergeMessage(message.CInject, itByte)
			go networks.TcpDial(send_msg, d.Ip_nodeTable[uint64(utils.Addr2Shard(from))][0])

			UUID := hexString
			now := time.Now()
			var tx message.TxInfo
			var ok bool
			for time.Since(now).Seconds() < 30 {
				TxInfoMapLock.Lock()
				tx, ok = TxInfoMap[UUID]
				if ok {
					TxInfoMapLock.Unlock()
					break
				}
				TxInfoMapLock.Unlock()
				time.Sleep(time.Millisecond * 100)
			}

			response.Result = "0x" + hex.EncodeToString(tx.Res)

		case "eth_getCode":
			acc := request.Params[0].(string)[2:]

			global.AccCodeMapLock.Lock()
			delete(global.AccCodeMap, acc)
			global.AccCodeMapLock.Unlock()
			m := new(message.QueryAccount)
			m.FromNodeID = 0
			m.FromShardID = params.DeciderShard
			m.Account = acc
			b, _ := json.Marshal(m)
			ms := message.MergeMessage(message.CQueryAccount, b)
			go networks.TcpDial(ms, d.Ip_nodeTable[uint64(utils.Addr2Shard(acc))][0])
			var Code []byte
			for true {
				global.AccCodeMapLock.Lock()
				if code, ok := global.AccCodeMap[acc]; ok {
					Code = code
					delete(global.AccCodeMap, acc)
					global.AccCodeMapLock.Unlock()
					break
				} else {
					global.AccCodeMapLock.Unlock()
					time.Sleep(time.Millisecond * 100)
				}
			}

			response.Result = "0x" + hex.EncodeToString(Code)

		default:
			response.Error = map[string]interface{}{"code": -32601, "message": "Method not found"}
		}

		c.JSON(http.StatusOK, response)
	})
	//http://172.27.30.106:8545

	router := r.Group("/broker-fi")

	router.GET("/querynodeinfo", func(c *gin.Context) {
		res := make(map[string]message.NodeInfo)
		NodeInfoMapLock.Lock()
		for k, v := range NodeInfoMap {
			res[k] = v
		}

		NodeInfoMapLock.Unlock()

		c.JSON(http.StatusOK, res)
	})

	router.GET("/querytxinfo", func(c *gin.Context) {
		res := make(map[string]message.TxInfo)
		NodeInfoMapLock.Lock()
		for k, v := range TxInfoMap {
			res[k] = v
		}

		NodeInfoMapLock.Unlock()

		c.JSON(http.StatusOK, res)
	})

	//接收钱包节点发送的交易，交给B2E处理
	router.POST("/sendTxtoB2E", service.SendTx2Network)

	//查询broker在各分片的收益
	router.GET("/querybrokerprofit", func(c *gin.Context) {
		d := c.MustGet("supervisor").(*Supervisor)
		addr := c.Query("addr")
		if addr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Address is required"})
			return
		}
		fmt.Println("addr:", addr)
		d.ComMod.(*committee.BrokerCommitteeMod_b2e).BrokerBalanceLock.Lock()
		defer d.ComMod.(*committee.BrokerCommitteeMod_b2e).BrokerBalanceLock.Unlock()

		m := d.ComMod.(*committee.BrokerCommitteeMod_b2e).Broker.ProfitBalance[addr]
		m2 := d.ComMod.(*committee.BrokerCommitteeMod_b2e).Broker.BrokerBalance[addr]
		m3 := d.ComMod.(*committee.BrokerCommitteeMod_b2e).Broker.LockBalance[addr]
		convertedData := make(map[string]string)
		for k, v := range m2 {
			convertedData[fmt.Sprintf("%d", k)] = m[k].String() + "/" + (new(big.Int).Add(v, m3[k])).String()
		}

		c.JSON(http.StatusOK, convertedData)

	})

	router.GET("/querybrokerprofit2", func(c *gin.Context) {
		d := c.MustGet("supervisor").(*Supervisor)
		addr := c.Query("addr")
		if addr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Address is required"})
			return
		}
		fmt.Println("addr:", addr)
		d.ComMod.(*committee.BrokerCommitteeMod_b2e).BrokerBalanceLock.Lock()
		defer d.ComMod.(*committee.BrokerCommitteeMod_b2e).BrokerBalanceLock.Unlock()

		m := d.ComMod.(*committee.BrokerCommitteeMod_b2e).Broker.ProfitBalance[addr]
		m2 := d.ComMod.(*committee.BrokerCommitteeMod_b2e).Broker.BrokerBalance[addr]
		m3 := d.ComMod.(*committee.BrokerCommitteeMod_b2e).Broker.LockBalance[addr]
		convertedData := make(map[string]string)
		for k, v := range m2 {
			convertedData[fmt.Sprintf("%d", k)] = m[k].String() + "/" + m3[k].String() + "/" + v.String()
		}

		c.JSON(http.StatusOK, convertedData)

	})

	//查询是否是broker
	router.GET("/queryisbroker", func(c *gin.Context) {
		d := c.MustGet("supervisor").(*Supervisor)
		addr := c.Query("addr")
		if addr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Address is required"})
			return
		}
		fmt.Println("addr:", addr)
		d.ComMod.(*committee.BrokerCommitteeMod_b2e).BrokerBalanceLock.Lock()
		isBroker := d.ComMod.(*committee.BrokerCommitteeMod_b2e).Broker.IsBroker(addr)
		d.ComMod.(*committee.BrokerCommitteeMod_b2e).BrokerBalanceLock.Unlock()
		if isBroker {
			c.JSON(http.StatusOK, gin.H{"is_broker": "true"})
		} else {
			c.JSON(http.StatusOK, gin.H{"is_broker": "false"})
		}
	})

	var QueryLock sync.Mutex

	//查询账户余额
	router.GET("/query-g", func(c *gin.Context) {
		QueryLock.Lock()
		defer QueryLock.Unlock()
		addr := c.Query("addr")
		if addr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Address is required"})
			return
		}
		fmt.Println("addr:", addr)
		//Clt.AccountStateMapLock.Lock()
		//delete(Clt.AccountStateRequestMap, addr)
		//Clt.AccountStateMapLock.Unlock()
		//Clt.SendAccountStateRequest2Worker(addr)
		//ret := "Try for 10s... Cannot access blockEmulator Network."
		//beginRequestTime := time.Now()
		//for time.Since(beginRequestTime) < time.Second*10 {
		//	Clt.AccountStateMapLock.Lock()
		//	if state, ok := Clt.AccountStateRequestMap[addr]; ok {
		//		r := ReturnAccountState{
		//			AccountAddr: addr,
		//			Balance:     state.Balance.String(),
		//		}
		//		c.JSON(http.StatusOK, r)
		//		Clt.AccountStateMapLock.Unlock()
		//		return
		//	}
		//	Clt.AccountStateMapLock.Unlock()
		//	time.Sleep(time.Millisecond * 30)
		//}

		m := new(message.QueryAccount)
		m.FromNodeID = 0
		m.FromShardID = params.DeciderShard
		m.Account = addr
		global.AccBalanceMapLock.Lock()
		delete(global.AccBalanceMap, addr)
		global.AccBalanceMapLock.Unlock()
		b, _ := json.Marshal(m)
		ms := message.MergeMessage(message.CQueryAccount, b)
		go networks.TcpDial(ms, d.Ip_nodeTable[uint64(utils.Addr2Shard(addr))][0])
		var Balance uint64
		for true {
			global.AccBalanceMapLock.Lock()
			if balance, ok := global.AccBalanceMap[addr]; ok {
				Balance = balance
				delete(global.AccBalanceMap, addr)
				global.AccBalanceMapLock.Unlock()
				break
			} else {
				global.AccBalanceMapLock.Unlock()
				time.Sleep(time.Millisecond * 100)
			}
		}

		n := big.NewInt(int64(Balance))

		r := ReturnAccountState{
			AccountAddr: addr,
			Balance:     n.String(),
		}
		//wei, _ := big.NewInt(0).SetString("1000000000000000000", 10)
		//n.Mul(n, wei)

		c.JSON(http.StatusOK, r)
		//c.JSON(http.StatusOK, gin.H{"error": ret})
	})

	//router.GET("/applybroker", func(c *gin.Context) {
	//	time.Sleep(time.Second*1)
	//	addr := c.Query("addr")
	//	d := c.MustGet("supervisor").(*Supervisor)
	//
	//	broker := d.ComMod.(*committee.BrokerCommitteeMod_b2e).Broker
	//	d.ComMod.(*committee.BrokerCommitteeMod_b2e).BrokerBalanceLock.Lock()
	//	if !broker.IsBroker(addr) {
	//		broker.BrokerAddress = append([]string{addr}, broker.BrokerAddress...)
	//
	//		broker.BrokerBalance[addr] = make(map[uint64]*big.Int)
	//		for sid := uint64(0); sid < uint64(params.ShardNum); sid++ {
	//			broker.BrokerBalance[addr][sid] = new(big.Int).Set(big.NewInt(0))
	//		}
	//		broker.LockBalance[addr] = make(map[uint64]*big.Int)
	//		for sid := uint64(0); sid < uint64(params.ShardNum); sid++ {
	//			broker.LockBalance[addr][sid] = new(big.Int).Set(big.NewInt(0))
	//		}
	//
	//		broker.ProfitBalance[addr] = make(map[uint64]*big.Float)
	//		for sid := uint64(0); sid < uint64(params.ShardNum); sid++ {
	//			broker.ProfitBalance[addr][sid] = new(big.Float).Set(big.NewFloat(0))
	//		}
	//	}
	//	d.ComMod.(*committee.BrokerCommitteeMod_b2e).BrokerBalanceLock.Unlock()
	//	c.JSON(http.StatusOK, gin.H{"message": "申请成为Broker成功!"})
	//})

	router.GET("/applybroker", func(c *gin.Context) {

		addr := c.Query("addr")
		d := c.MustGet("supervisor").(*Supervisor)
		if addr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Address is required"})
			return
		}

		//for sid := uint64(0); sid < uint64(params.ShardNum); sid++ {
		//	input1, _ := hex.DecodeString("c0d7fdac")
		//	executeContractTransaction := core.NewTransactionContractBroker(addr, d.contractCreationMap[sid], big.NewInt(0), 1, input1)
		//	executeContractTransaction.Gas = 500000
		//	executeContractTransaction.GasPrice = big.NewInt(1)
		//	var txs = []*core.Transaction{executeContractTransaction}
		//	it := message.InjectTxs{
		//		Txs:       txs,
		//		ToShardID: sid,
		//	}
		//	itByte, err := json.Marshal(it)
		//	if err != nil {
		//		log.Panic(err)
		//	}
		//	send_msg := message.MergeMessage(message.CInject, itByte)
		//	go networks.TcpDial(send_msg, d.Ip_nodeTable[sid][0])
		//}
		//
		//required_cnt := 2 * ((d.ChainConfig.ShardNums - 1) / 3)
		//f := false
		//now := time.Now()
		//for time.Since(now) < time.Second*15 {
		//	d.contractAllowMapMutex.Lock()
		//	count := uint64(0)
		//	for _, v := range d.contractAllowMap {
		//		if _, exists := v[addr]; exists {
		//			count++
		//		}
		//	}
		//	d.contractAllowMapMutex.Unlock()
		//	if count >= required_cnt {
		//		f = true
		//		break
		//	}
		//	time.Sleep(time.Millisecond * 100)
		//}
		//if f {
		broker := d.ComMod.(*committee.BrokerCommitteeMod_b2e).Broker
		d.ComMod.(*committee.BrokerCommitteeMod_b2e).BrokerBalanceLock.Lock()
		if !broker.IsBroker(addr) {
			broker.BrokerAddress = append([]string{addr}, broker.BrokerAddress...)

			broker.BrokerBalance[addr] = make(map[uint64]*big.Int)
			for sid := uint64(0); sid < uint64(params.ShardNum); sid++ {
				broker.BrokerBalance[addr][sid] = new(big.Int).Set(big.NewInt(0))
			}
			broker.LockBalance[addr] = make(map[uint64]*big.Int)
			for sid := uint64(0); sid < uint64(params.ShardNum); sid++ {
				broker.LockBalance[addr][sid] = new(big.Int).Set(big.NewInt(0))
			}

			broker.ProfitBalance[addr] = make(map[uint64]*big.Float)
			for sid := uint64(0); sid < uint64(params.ShardNum); sid++ {
				broker.ProfitBalance[addr][sid] = new(big.Float).Set(big.NewFloat(0))
			}
		}
		d.ComMod.(*committee.BrokerCommitteeMod_b2e).BrokerBalanceLock.Unlock()

		c.JSON(http.StatusOK, gin.H{"message": "申请成为Broker成功!"})
		//} else {
		//	c.JSON(http.StatusOK, gin.H{"error": "申请成为Broker失败！"})
		//}
		return
	})

	//申请成为broker/质押更多的钱
	router.GET("BecomeBrokerOrStakeMore", func(c *gin.Context) {
		d := c.MustGet("supervisor").(*Supervisor)
		addr := c.Query("addr")

		d.ComMod.(*committee.BrokerCommitteeMod_b2e).BrokerBalanceLock.Lock()
		isBroker := d.ComMod.(*committee.BrokerCommitteeMod_b2e).Broker.IsBroker(addr)
		if !isBroker {
			c.JSON(http.StatusOK, gin.H{"error": "not a broker,cannot invoke Stake!"})
			d.ComMod.(*committee.BrokerCommitteeMod_b2e).BrokerBalanceLock.Unlock()
			return
		}
		d.ComMod.(*committee.BrokerCommitteeMod_b2e).BrokerBalanceLock.Unlock()

		token := c.Query("token") //申请质押的钱
		if addr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Address is required"})
			return
		}
		if token == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Amount is required"})
			return
		}
		tokenInt, success := new(big.Int).SetString(token, 10)
		if !success {
			c.JSON(http.StatusOK, gin.H{"error": "Amount is invalid!"})
			return
		}
		if tokenInt.Cmp(big.NewInt(0)) <= 0 {
			c.JSON(http.StatusOK, gin.H{"error": "Amount is invalid!"})
			return
		}
		balancepershard := new(big.Int).Div(tokenInt, new(big.Int).SetInt64(int64(params.ShardNum)))

		if balancepershard.Cmp(big.NewInt(0)) == 0 {
			c.JSON(http.StatusOK, gin.H{"error": "Please stake more tokens"})
			return
		}

		d.ComMod.(*committee.BrokerCommitteeMod_b2e).LastInvokeTimeMutex.Lock()
		invokeTime := d.ComMod.(*committee.BrokerCommitteeMod_b2e).LastInvokeTime
		if lastInvokeTime, exist := invokeTime[addr]; exist {
			if time.Since(lastInvokeTime) < 15*time.Second {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Stake request too frequent! Please wait a moment !"})
				d.ComMod.(*committee.BrokerCommitteeMod_b2e).LastInvokeTimeMutex.Unlock()
				return
			}
			d.ComMod.(*committee.BrokerCommitteeMod_b2e).LastInvokeTime[addr] = time.Now()
		} else {
			d.ComMod.(*committee.BrokerCommitteeMod_b2e).LastInvokeTime[addr] = time.Now()
		}
		d.ComMod.(*committee.BrokerCommitteeMod_b2e).LastInvokeTimeMutex.Unlock()

		fmt.Println(addr)
		fmt.Println(token)

		//查询账户余额
		Clt.AccountStateMapLock.Lock()
		delete(Clt.AccountStateRequestMap, addr)
		Clt.AccountStateMapLock.Unlock()
		Clt.SendAccountStateRequest2Worker(addr)
		//ret := "Try for 10s... Cannot access blockEmulator Network."
		beginRequestTime := time.Now()
		var balance *big.Int
		for time.Since(beginRequestTime) < time.Second*10 {
			Clt.AccountStateMapLock.Lock()
			if state, ok := Clt.AccountStateRequestMap[addr]; ok {
				balance = state.Balance
				Clt.AccountStateMapLock.Unlock()
				break
			}
			Clt.AccountStateMapLock.Unlock()
			time.Sleep(time.Millisecond * 30)
		}
		if balance == nil {
			fmt.Println("balance is nil")
			c.JSON(http.StatusOK, gin.H{"error": "balance is nil"})
			return
		}
		fmt.Println("balnce is ", balance)

		//校验余额要大于等于质押的token
		if balance.Uint64() < tokenInt.Uint64() {
			c.JSON(http.StatusOK, gin.H{"error": "Your balance is less than the required stake amount."})
			return
		}

		tokenInt = new(big.Int).Mul(balancepershard, new(big.Int).SetInt64(int64(params.ShardNum)))
		//扣减余额
		tx := core.NewTransaction(addr, addr, new(big.Int).Abs(tokenInt), uint64(123), big.NewInt(0))
		tx.IsAllocatedSender = true
		txs := make([]*core.Transaction, 0)
		txs = append(txs, tx)
		it := message.InjectTxs{
			Txs:       txs,
			ToShardID: Clt.GetAddr2ShardMap(addr),
		}
		itByte, err2 := json.Marshal(it)
		if err2 != nil {
			log.Panic(err2)
		}
		send_msg := message.MergeMessage(message.CInjectHead, itByte)
		go networks.TcpDial(send_msg, d.ComMod.(*committee.BrokerCommitteeMod_b2e).IpNodeTable[Clt.GetAddr2ShardMap(addr)][0])

		d.ComMod.(*committee.BrokerCommitteeMod_b2e).BrokerBalanceLock.Lock()
		defer d.ComMod.(*committee.BrokerCommitteeMod_b2e).BrokerBalanceLock.Unlock()

		//添加各分片余额到broker数据结构中

		broker := d.ComMod.(*committee.BrokerCommitteeMod_b2e).Broker
		if !broker.IsBroker(addr) {
			broker.BrokerAddress = append([]string{addr}, broker.BrokerAddress...)

			broker.BrokerBalance[addr] = make(map[uint64]*big.Int)
			for sid := uint64(0); sid < uint64(params.ShardNum); sid++ {
				broker.BrokerBalance[addr][sid] = new(big.Int).Set(balancepershard)
			}
			broker.LockBalance[addr] = make(map[uint64]*big.Int)
			for sid := uint64(0); sid < uint64(params.ShardNum); sid++ {
				broker.LockBalance[addr][sid] = new(big.Int).Set(big.NewInt(0))
			}

			broker.ProfitBalance[addr] = make(map[uint64]*big.Float)
			for sid := uint64(0); sid < uint64(params.ShardNum); sid++ {
				broker.ProfitBalance[addr][sid] = new(big.Float).Set(big.NewFloat(0))
			}

		} else {
			for sid := uint64(0); sid < uint64(params.ShardNum); sid++ {
				broker.BrokerBalance[addr][sid] = new(big.Int).Add(broker.BrokerBalance[addr][sid], balancepershard)
			}
		}

		c.JSON(http.StatusOK, gin.H{"message": "Successfully stake " + tokenInt.String() + " tokens to B2E"})

	})

	router.GET("withdrawbroker", func(c *gin.Context) {
		d := c.MustGet("supervisor").(*Supervisor)
		addr := c.Query("addr")
		if addr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Address is required"})
			return
		}
		fmt.Println("addr:", addr)

		d.ComMod.(*committee.BrokerCommitteeMod_b2e).BrokerBalanceLock.Lock()
		defer d.ComMod.(*committee.BrokerCommitteeMod_b2e).BrokerBalanceLock.Unlock()

		isBroker := d.ComMod.(*committee.BrokerCommitteeMod_b2e).Broker.IsBroker(addr)
		if !isBroker {
			c.JSON(http.StatusOK, gin.H{"error": "not a broker,cannot invoke withdrawbroker!"})
			return
		}

		//计算需要返还的钱
		var profit *big.Float
		profit = new(big.Float).SetFloat64(0)
		for sid := uint64(0); sid < uint64(params.ShardNum); sid++ {
			profit = new(big.Float).Add(new(big.Float).SetInt(d.ComMod.(*committee.BrokerCommitteeMod_b2e).Broker.BrokerBalance[addr][sid]), profit)
		}
		for sid := uint64(0); sid < uint64(params.ShardNum); sid++ {
			profit = new(big.Float).Add(new(big.Float).SetInt(d.ComMod.(*committee.BrokerCommitteeMod_b2e).Broker.LockBalance[addr][sid]), profit)
		}
		for sid := uint64(0); sid < uint64(params.ShardNum); sid++ {
			profit = new(big.Float).Add(d.ComMod.(*committee.BrokerCommitteeMod_b2e).Broker.ProfitBalance[addr][sid], profit)
		}
		profitUint64, _ := profit.Uint64()

		//返还到账户余额
		tx := core.NewTransaction(addr, addr, new(big.Int).SetUint64(profitUint64), uint64(123), big.NewInt(0))
		tx.IsAllocatedRecipent = true
		txs := make([]*core.Transaction, 0)
		txs = append(txs, tx)
		it := message.InjectTxs{
			Txs:       txs,
			ToShardID: Clt.GetAddr2ShardMap(addr),
		}
		itByte, err2 := json.Marshal(it)
		if err2 != nil {
			log.Panic(err2)
		}
		send_msg := message.MergeMessage(message.CInjectHead, itByte)
		go networks.TcpDial(send_msg, d.ComMod.(*committee.BrokerCommitteeMod_b2e).IpNodeTable[Clt.GetAddr2ShardMap(addr)][0])

		//从broker数据结构中删除该地址
		d.ComMod.(*committee.BrokerCommitteeMod_b2e).Broker.BrokerAddress = removeElement(d.ComMod.(*committee.BrokerCommitteeMod_b2e).Broker.BrokerAddress, addr)
		delete(d.ComMod.(*committee.BrokerCommitteeMod_b2e).Broker.BrokerBalance, addr)
		delete(d.ComMod.(*committee.BrokerCommitteeMod_b2e).Broker.LockBalance, addr)
		delete(d.ComMod.(*committee.BrokerCommitteeMod_b2e).Broker.ProfitBalance, addr)

		c.JSON(http.StatusOK, gin.H{"message": "Successfully withdraw " + new(big.Int).SetUint64(profitUint64).String() + " tokens from B2E"})
	})

	//TODO
	router.GET("queryTransactionByUserAddr", func(c *gin.Context) {
		//d := c.MustGet("supervisor").(*Supervisor)
		Clt := c.MustGet("clt").(*client.Client)
		addr := c.Query("addr")
		if addr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Address is required"})
			return
		}

		crtx := &message.ClientRequestTransaction{
			AccountAddr: addr,
			ClientIp:    Clt.IpAddr,
		}
		crtxByte, err := json.Marshal(crtx)
		if err != nil {
			log.Panic(err)
		}
		send_msg := message.MergeMessage(message.CClientRequestTransaction, crtxByte)

		for sid := uint64(0); sid < uint64(params.ShardNum); sid++ {
			go networks.TcpDial(send_msg, params.IPmap_nodeTable[sid][0])
		}

	})
	fmt.Println(fmt.Sprintf(getMyIp()+":%d", 8545))

	//r.Run(fmt.Sprintf(getMyIp()+":%d", 8545))
	//return r.Run("127.0.0.1:8545")
	err := http.ListenAndServe(":8545", r)
	if err != nil {
		return err
	}
	return nil
}
func removeElement(slice []string, element string) []string {
	// 遍历切片，找到元素的索引
	for i, v := range slice {
		if v == element {
			// 使用切片操作删除元素
			// slice[:i] 表示从切片开始到索引 i（不包括 i）的部分
			// slice[i+1:] 表示从索引 i+1 到切片结束的部分
			// 将这两部分连接起来，就得到了删除了指定元素的切片
			return append(slice[:i], slice[i+1:]...)
		}
	}
	// 如果元素不在切片中，返回原切片
	return slice
}

type ReturnAccountState struct {
	AccountAddr string `json:"account"`
	Balance     string `json:"balance"`
}

func getMyIp() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	for _, addr := range addrs {
		var ip net.IP

		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}

		// 过滤掉 IPv6 地址和回环地址（127.0.0.1）
		if ip.To4() != nil && !ip.IsLoopback() {
			fmt.Println("IP Address:", ip.String())
			if ip.To4()[0] == 0xac {
				return ip.String()
			}
			//if ip.To4()[0] == 0xc0 {
			//	return ip.String()
			//}
		}
	}
	return "127.0.0.1"
}

// 解决跨域问题
func CorsConfig() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*") // 可将将 * 替换为指定的域名
		c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")
		c.Header("Access-Control-Allow-Headers", "*")
		c.Header("Access-Control-Expose-Headers", "*")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(200)
		} else {
			c.Next()
		}
	}
}
func (d *Supervisor) handleResponseAcc(content []byte) {
	msg := new(message.AccountRes)
	err := json.Unmarshal(content, msg)
	if err != nil {
		log.Panic(err)
	}
	global.AccBalanceMapLock.Lock()
	global.AccBalanceMap[msg.Account] = msg.Balance
	global.AccBalanceMapLock.Unlock()

	global.AccCodeMapLock.Lock()
	global.AccCodeMap[msg.Account] = msg.Code
	global.AccCodeMapLock.Unlock()

}

// handle message. only one message to be handled now
func (d *Supervisor) handleMessage(msg []byte) {
	msgType, content := message.SplitMessage(msg)
	switch msgType {
	case message.CBlockInfo:
		d.handleBlockInfos(content)
	case message.CContractCreateSuccess:
		d.handleContractCreateSuccess(content)
	case message.CContractExecuteSuccess:
		d.handleContractExecuteSuccess(content)
	case message.CNodeInfo:
		d.handleNodeInfo(content)
	case message.CTxInfo:
		d.handleTxInfo(content)
	case message.CQueryContractResultRes:
		d.handleQueryContractResultRes(content)
	case message.CResponseAccount:
		go d.handleResponseAcc(content)

		// add codes for more functionality
	default:
		d.ComMod.HandleOtherMessage(msg)
		for _, mm := range d.testMeasureMods {
			mm.HandleExtraMessage(msg)
		}
	}
}

var QueryContractResultResMap = make(map[string]chan []byte)

func (d *Supervisor) handleQueryContractResultRes(content []byte) {
	m := new(message.QueryContractResultRes)
	json.Unmarshal(content, m)
	if m.Success {

		s := hex.EncodeToString(m.Result)
		fmt.Println("查询合约执行结果成功，uuid为" + m.UUID + ",结果为：" + s)

		QueryContractResultResMap[m.UUID] <- m.Result
	} else {
		fmt.Println("查询失败：" + m.UUID)
	}
}

func (d *Supervisor) handleClientRequest(con net.Conn) {
	defer con.Close()
	clientReader := bufio.NewReader(con)
	for {
		clientRequest, err := clientReader.ReadBytes('\n')
		switch err {
		case nil:
			d.tcpLock.Lock()
			d.handleMessage(clientRequest)
			d.tcpLock.Unlock()
		case io.EOF:
			log.Println("client closed the connection by terminating the process")
			return
		default:
			log.Printf("error: %v\n", err)
			return
		}
	}
}

func (d *Supervisor) TcpListen() {
	ln, err := net.Listen("tcp", d.IPaddr)
	if err != nil {
		log.Panic(err)
	}
	d.tcpLn = ln
	for {
		conn, err := d.tcpLn.Accept()
		if err != nil {
			return
		}
		go d.handleClientRequest(conn)
	}
}

// tcp listen for Supervisor
func (d *Supervisor) OldTcpListen() {
	ipaddr, err := net.ResolveTCPAddr("tcp", d.IPaddr)
	if err != nil {
		log.Panic(err)
	}
	ln, err := net.ListenTCP("tcp", ipaddr)
	d.tcpLn = ln
	if err != nil {
		log.Panic(err)
	}
	d.sl.Slog.Printf("Supervisor begins listening：%s\n", d.IPaddr)

	for {
		conn, err := d.tcpLn.Accept()
		if err != nil {
			if d.listenStop {
				return
			}
			log.Panic(err)
		}
		b, err := io.ReadAll(conn)
		if err != nil {
			log.Panic(err)
		}
		d.handleMessage(b)
		conn.(*net.TCPConn).SetLinger(0)
		defer conn.Close()
	}
}

// close Supervisor, and record the data in .csv file
func (d *Supervisor) CloseSupervisor() {
	d.sl.Slog.Println("Closing...")
	for _, measureMod := range d.testMeasureMods {
		d.sl.Slog.Println(measureMod.OutputMetricName())
		d.sl.Slog.Println(measureMod.OutputRecord())
		println()
	}

	d.ComMod.Result_save()

	d.sl.Slog.Println("Trying to input .csv")
	// write to .csv file
	dirpath := params.DataWrite_path + "supervisor_measureOutput/"
	err := os.MkdirAll(dirpath, os.ModePerm)
	if err != nil {
		log.Panic(err)
	}
	for _, measureMod := range d.testMeasureMods {
		targetPath := dirpath + measureMod.OutputMetricName() + ".csv"
		f, err := os.Open(targetPath)
		resultPerEpoch, totResult := measureMod.OutputRecord()
		resultStr := make([]string, 0)
		for _, result := range resultPerEpoch {
			resultStr = append(resultStr, strconv.FormatFloat(result, 'f', 8, 64))
		}
		resultStr = append(resultStr, strconv.FormatFloat(totResult, 'f', 8, 64))
		if err != nil && os.IsNotExist(err) {
			file, er := os.Create(targetPath)
			if er != nil {
				panic(er)
			}
			defer file.Close()

			w := csv.NewWriter(file)
			title := []string{measureMod.OutputMetricTitle()}
			w.Write(title)
			w.Flush()
			w.Write(resultStr)
			w.Flush()
		} else {
			file, err := os.OpenFile(targetPath, os.O_APPEND|os.O_RDWR, 0666)

			if err != nil {
				log.Panic(err)
			}
			defer file.Close()
			writer := csv.NewWriter(file)
			err = writer.Write(resultStr)
			if err != nil {
				log.Panic()
			}
			writer.Flush()
		}
		f.Close()
		d.sl.Slog.Println(measureMod.OutputRecord())
	}
	networks.CloseAllConnInPool()
	d.tcpLn.Close()
}

var NodeInfoMapLock sync.Mutex
var NodeInfoMap = make(map[string]message.NodeInfo)

func (d *Supervisor) handleNodeInfo(content []byte) {
	s := new(message.NodeInfo)
	err := json.Unmarshal(content, s)
	if err != nil {
		log.Panic()
	}
	key := fmt.Sprintf("S%dN%d", s.ShardId, s.NodeId)
	NodeInfoMapLock.Lock()
	NodeInfoMap[key] = *s
	NodeInfoMapLock.Unlock()

}

var TxInfoMapLock sync.Mutex
var TxInfoMap = make(map[string]message.TxInfo)

func (d *Supervisor) handleTxInfo(content []byte) {
	s := new(message.TxInfo)
	err := json.Unmarshal(content, s)
	if err != nil {
		log.Panic()
	}
	TxInfoMapLock.Lock()

	TxInfoMap[base64.StdEncoding.EncodeToString(s.TxHash)] = *s

	TxInfoMap[s.UUID] = *s
	TxInfoMapLock.Unlock()

}

func (d *Supervisor) handleContractCreateSuccess(content []byte) {
	s := new(message.ContractCreateSuccess)
	err := json.Unmarshal(content, s)
	if err != nil {
		log.Panic()
	}

	d.contractCreationMapMutex.Lock()
	defer d.contractCreationMapMutex.Unlock()
	d.contractCreationMap[s.ShardId] = s.Addr
}

func (d *Supervisor) handleContractExecuteSuccess(content []byte) {
	s := new(message.ContractExecuteSuccess)
	err := json.Unmarshal(content, s)
	if err != nil {
		log.Panic()
	}

	d.contractAllowMapMutex.Lock()
	defer d.contractAllowMapMutex.Unlock()
	d.contractAllowMap[s.ShardId][s.Addr] = struct{}{}
}
