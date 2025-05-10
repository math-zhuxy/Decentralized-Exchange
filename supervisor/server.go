package supervisor

import (
	"blockEmulator/client"
	"blockEmulator/core"
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
	"time"

	"slices"

	"github.com/gin-gonic/gin"
)

func (d *Supervisor) queryForAccBalance(addr string, tx_type string) *big.Int {
	msg := &message.AccountBalanceQuery{
		Addr:   addr,
		TXType: tx_type,
	}
	msg_bytes, _ := json.Marshal(msg)
	final_msg := message.MergeMessage(message.DAccountBalanceQuery, msg_bytes)
	go networks.TcpDial(final_msg, d.Ip_nodeTable[uint64(utils.Addr2Shard(addr))][0])
	acc_bal := new(big.Int)
	for {
		time.Sleep(time.Microsecond * 100)
		d.accBalanceMapLock.Lock()
		info := d.accountBalanceMap[tx_type][addr]
		if info == nil {
			d.accBalanceMapLock.Unlock()
			continue
		}
		if info.isLatest {
			acc_bal.Set(info.balance)
			info.isLatest = false
			d.accountBalanceMap[tx_type][addr] = info
			d.accBalanceMapLock.Unlock()
			break
		} else {
			d.accBalanceMapLock.Unlock()
		}
	}
	return acc_bal
}

func (d *Supervisor) RunHTTP() error {
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
		tx_type := c.Query("type")
		if !slices.Contains(params.Transaction_Types, tx_type) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Tx Type"})
			return
		}
		if addr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Address is required"})
			return
		}
		fmt.Println("addr:", addr)
		d.ComMod.(*committee.BrokerCommitteeMod_b2e).BrokerBalanceLock.Lock()
		defer d.ComMod.(*committee.BrokerCommitteeMod_b2e).BrokerBalanceLock.Unlock()

		m := d.ComMod.(*committee.BrokerCommitteeMod_b2e).Broker.ProfitBalance[tx_type][addr]
		m2 := d.ComMod.(*committee.BrokerCommitteeMod_b2e).Broker.BrokerBalance[tx_type][addr]
		m3 := d.ComMod.(*committee.BrokerCommitteeMod_b2e).Broker.LockBalance[tx_type][addr]
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

	// 查询账户余额
	router.GET("/query-g", func(c *gin.Context) {
		addr := c.Query("addr")
		if addr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Address is required"})
			return
		}
		tx_type := c.Query("type")
		if tx_type == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Type is required"})
			return
		}
		if !slices.Contains(params.Transaction_Types, tx_type) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Tx Type"})
			return
		}

		n := d.queryForAccBalance(addr, tx_type)

		r := ReturnAccountState{
			AccountAddr: addr,
			Balance:     n.String(),
		}

		c.JSON(http.StatusOK, r)
	})

	router.GET("/exchange", func(c *gin.Context) {
		addr := c.Query("addr")
		d := c.MustGet("supervisor").(*Supervisor)
		token := c.Query("token")
		tx_type_from := c.Query("type_from")
		tx_type_to := c.Query("type_to")

		if addr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Address is required"})
			return
		}
		if tx_type_from == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Type From is required"})
			return
		}
		if tx_type_to == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Type To is required"})
			return
		}
		if !slices.Contains(params.Transaction_Types, tx_type_from) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Tx From Type"})
			return
		}
		if !slices.Contains(params.Transaction_Types, tx_type_to) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Tx To Type"})
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
		given_money := new(big.Int)
		if tx_type_from == "BKC" {
			d.amm_model.bkc.Add(d.amm_model.bkc, tokenInt)
			rest_badge := new(big.Int).Quo(d.amm_model.constant_val, d.amm_model.bkc)
			given_money.Sub(d.amm_model.badge, rest_badge)
			d.amm_model.badge.Set(rest_badge)
		} else if tx_type_from == "Badge" {
			d.amm_model.badge.Add(d.amm_model.badge, tokenInt)
			rest_bkc := new(big.Int).Quo(d.amm_model.constant_val, d.amm_model.badge)
			given_money.Sub(d.amm_model.badge, rest_bkc)
			d.amm_model.bkc.Set(rest_bkc)
		} else {
			log.Panic()
		}
		tx := core.NewTransaction(addr, addr, tokenInt, uint64(123), big.NewInt(0), tx_type_from)
		tx.ShouldHandleInBlock = true
		tx.IncreaseOrDecrease = 1
		tx_2 := core.NewTransaction(addr, addr, given_money, uint64(123), big.NewInt(0), tx_type_to)
		tx_2.ShouldHandleInBlock = true
		tx_2.IncreaseOrDecrease = 2
		txs := make([]*core.Transaction, 0)
		txs = append(txs, tx)
		txs = append(txs, tx_2)
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
		c.JSON(http.StatusOK, gin.H{"msg": "Done!"})
	})

	router.GET("/applybroker", func(c *gin.Context) {

		addr := c.Query("addr")
		d := c.MustGet("supervisor").(*Supervisor)
		token := c.Query("token")
		tx_type := c.Query("type")
		if addr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Address is required"})
			return
		}
		if tx_type == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Type is required"})
			return
		}
		if !slices.Contains(params.Transaction_Types, tx_type) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Tx Type"})
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
		tx := core.NewTransaction(addr, addr, new(big.Int).Abs(tokenInt), uint64(123), big.NewInt(0), tx_type)
		tx.ShouldHandleInBlock = true
		tx.IncreaseOrDecrease = 1
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
		balancepershard := new(big.Int).Div(tokenInt, new(big.Int).SetInt64(int64(params.ShardNum)))

		if balancepershard.Cmp(big.NewInt(0)) == 0 {
			c.JSON(http.StatusOK, gin.H{"error": "Please stake more tokens"})
			return
		}

		broker := d.ComMod.(*committee.BrokerCommitteeMod_b2e).Broker
		if broker.IsBroker(addr) {
			c.JSON(http.StatusOK, gin.H{"msg": "Already Broker"})
			return
		}
		d.ComMod.(*committee.BrokerCommitteeMod_b2e).BrokerBalanceLock.Lock()
		broker.BrokerAddress = append([]string{addr}, broker.BrokerAddress...)
		broker.BrokerBalance[tx_type][addr] = make(map[uint64]*big.Int)
		for sid := uint64(0); sid < uint64(params.ShardNum); sid++ {
			broker.BrokerBalance[tx_type][addr][sid] = new(big.Int).Set(balancepershard)
		}
		broker.LockBalance[tx_type][addr] = make(map[uint64]*big.Int)
		for sid := uint64(0); sid < uint64(params.ShardNum); sid++ {
			broker.LockBalance[tx_type][addr][sid] = new(big.Int).Set(big.NewInt(0))
		}

		broker.ProfitBalance[tx_type][addr] = make(map[uint64]*big.Float)
		for sid := uint64(0); sid < uint64(params.ShardNum); sid++ {
			broker.ProfitBalance[tx_type][addr][sid] = new(big.Float).Set(big.NewFloat(0))
		}

		d.ComMod.(*committee.BrokerCommitteeMod_b2e).BrokerBalanceLock.Unlock()

		c.JSON(http.StatusOK, gin.H{"message": "申请成为Broker成功!"})
	})

	router.GET("/withdrawbroker", func(c *gin.Context) {
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
		profitUint64 := uint64(0)
		for _, money_type := range params.Transaction_Types {
			if d.ComMod.(*committee.BrokerCommitteeMod_b2e).Broker.BrokerBalance[money_type][addr] == nil {
				continue
			}
			profit = new(big.Float).SetFloat64(0)
			for sid := uint64(0); sid < uint64(params.ShardNum); sid++ {
				profit = new(big.Float).Add(new(big.Float).SetInt(d.ComMod.(*committee.BrokerCommitteeMod_b2e).Broker.BrokerBalance[money_type][addr][sid]), profit)
			}
			for sid := uint64(0); sid < uint64(params.ShardNum); sid++ {
				profit = new(big.Float).Add(new(big.Float).SetInt(d.ComMod.(*committee.BrokerCommitteeMod_b2e).Broker.LockBalance[money_type][addr][sid]), profit)
			}
			for sid := uint64(0); sid < uint64(params.ShardNum); sid++ {
				profit = new(big.Float).Add(d.ComMod.(*committee.BrokerCommitteeMod_b2e).Broker.ProfitBalance[money_type][addr][sid], profit)
			}
			pf, _ := profit.Float64()
			profitUint64 += uint64(pf)
			//返还到账户余额
			tx := core.NewTransaction(addr, addr, new(big.Int).SetUint64(profitUint64), uint64(123), big.NewInt(0), money_type)
			tx.ShouldHandleInBlock = true
			tx.IncreaseOrDecrease = 2
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
			delete(d.ComMod.(*committee.BrokerCommitteeMod_b2e).Broker.BrokerBalance[money_type], addr)
			delete(d.ComMod.(*committee.BrokerCommitteeMod_b2e).Broker.LockBalance[money_type], addr)
			delete(d.ComMod.(*committee.BrokerCommitteeMod_b2e).Broker.ProfitBalance[money_type], addr)
		}

		//从broker数据结构中删除该地址
		d.ComMod.(*committee.BrokerCommitteeMod_b2e).Broker.BrokerAddress = removeElement(d.ComMod.(*committee.BrokerCommitteeMod_b2e).Broker.BrokerAddress, addr)

		c.JSON(http.StatusOK, gin.H{"message": "Successfully withdraw " + new(big.Int).SetUint64(profitUint64).String() + " tokens from B2E"})
	})

	fmt.Println(fmt.Sprintf(getMyIp()+":%d", 8545))
	err := http.ListenAndServe(":8545", r)
	if err != nil {
		return err
	}
	return nil
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
