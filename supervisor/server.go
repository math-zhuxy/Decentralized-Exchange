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
	"blockEmulator/utils"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

func (d *Supervisor) RunHTTP() error {

	go func() {
		for {
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

		c.JSON(http.StatusOK, r)
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

	fmt.Println(fmt.Sprintf(getMyIp()+":%d", 8545))
	err := http.ListenAndServe(":8545", r)
	if err != nil {
		return err
	}
	return nil
}
