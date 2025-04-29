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
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
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
var blockNumber = 0

func (d *Supervisor) RunHTTP() error {

	go func() {
		for {
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
		for {
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

	router.GET("/applybroker", func(c *gin.Context) {

		addr := c.Query("addr")
		d := c.MustGet("supervisor").(*Supervisor)
		if addr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Address is required"})
			return
		}
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
