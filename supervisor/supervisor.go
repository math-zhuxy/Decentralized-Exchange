// Supervisor is an abstract role in this simulator that may read txs, generate partition infos,
// and handle history data.

package supervisor

import (
	"blockEmulator/core"
	"blockEmulator/global"
	"blockEmulator/message"
	"blockEmulator/networks"
	"blockEmulator/params"
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
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type accBalMap struct {
	isLatest bool
	balance  *big.Int
}

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

	accountBalanceMap map[string]map[string]*accBalMap
	accBalanceMapLock sync.Mutex
}

func (d *Supervisor) NewSupervisor(ip string, pcc *params.ChainConfig, committeeMethod string, measureModNames ...string) {
	d.IPaddr = ip
	d.ChainConfig = pcc
	d.Ip_nodeTable = params.IPmap_nodeTable
	d.CommitteeMethod = committeeMethod
	d.sl = supervisor_log.NewSupervisorLog()

	accBalanceMap := make(map[string]map[string]*accBalMap)
	for _, tx_type := range params.Transaction_Types {
		accBalanceMap[tx_type] = make(map[string]*accBalMap)
	}
	d.accountBalanceMap = accBalanceMap

	d.Ss = signal.NewStopSignal(2 * int(pcc.ShardNums))
	d.contractCreationMap = make(map[uint64]string)
	d.contractAllowMap = make(map[uint64]map[string]struct{})
	for sid := uint64(0); sid < uint64(params.ShardNum); sid++ {
		d.contractAllowMap[sid] = make(map[string]struct{})
	}
	d.ComMod = committee.NewBrokerCommitteeMod_b2e(d.Ip_nodeTable, d.Ss, d.sl, params.FileInput, params.TotalDataSize, params.BatchSize)

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
}

// read transactions from dataFile. When the number of data is enough,
// the Supervisor will do re-partition and send partitionMSG and txs to leaders.

func (d *Supervisor) SupervisorTxHandling() {
	d.ComMod.MsgSendingControl()

	// 为了让 brokerchain 一直运行
	for true {
		time.Sleep(time.Second)
	}
}

const Broker2EarnAddr = "aaaa1a0ac6761fe4fcb6aca7dfa7e7c86e8dbc6d"

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
		}
	}
	return "127.0.0.1"
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
	case message.DAccountBalanceReply:
		go d.handleAccountBalReply(content)
		// add codes for more functionality
	default:
		d.ComMod.HandleOtherMessage(msg)
		for _, mm := range d.testMeasureMods {
			mm.HandleExtraMessage(msg)
		}
	}
}

var QueryContractResultResMap = make(map[string]chan []byte)

func (d *Supervisor) handleAccountBalReply(content []byte) {
	con := new(message.AccountBalanceReply)
	err := json.Unmarshal(content, con)
	if err != nil {
		log.Panic()
	}
	d.accBalanceMapLock.Lock()
	defer d.accBalanceMapLock.Unlock()
	accMap := new(accBalMap)
	accMap.isLatest = true
	accMap.balance = new(big.Int).Set(con.Balance)
	d.accountBalanceMap[con.TXType][con.Addr] = accMap
}

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

/*
处理来自客户端的 JSON-RPC 请求，根据请求的 method 字段识别调用的以太坊接口，并构造对应的响应。
该函数模拟了部分以太坊 JSON-RPC API（如 eth_getBlockByNumber, eth_getBalance, eth_sendTransaction 等），
用于区块链模拟器中测试或对接外部系统。
*/
func handleRpcRequest(d *Supervisor, request *RpcRequest) *RpcResponse {
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
			"0x107d73d8a49eeb85d32cf465507dd71d507100c1",
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

	return &response

}
