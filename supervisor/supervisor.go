// Supervisor is an abstract role in this simulator that may read txs, generate partition infos,
// and handle history data.

package supervisor

import (
	"blockEmulator/global"
	"blockEmulator/message"
	"blockEmulator/networks"
	"blockEmulator/params"
	"blockEmulator/supervisor/committee"
	"blockEmulator/supervisor/measure"
	"blockEmulator/supervisor/signal"
	"blockEmulator/supervisor/supervisor_log"
	"bufio"
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
