package service

import (
	"blockEmulator/client"
	"blockEmulator/core"
	"blockEmulator/mytool"
	"fmt"
	"github.com/gin-gonic/gin"
	"math/big"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type InputTransaction struct {
	From  string `json:"from" binding:"required"`
	To    string `json:"to" binding:"required"`
	Value string `json:"value" binding:"required"`
	Fee   string `json:"fee" binding:"required"`
}

var (
	LastInvokeTime      = make(map[string]time.Time)
	LastInvokeTimeMutex sync.Mutex
)

func SendTx2Network(c *gin.Context) {
	var it InputTransaction

	// 尝试绑定请求体到 it 结构
	if err := c.ShouldBind(&it); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// 将字符串 value 转换为 uint64
	uintValue, err := strconv.ParseUint(it.Value, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid value"})
		return
	}
	if uintValue <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid value"})
		return
	}

	// 将字符串 fee 转换为 uint64
	uintFee, err := strconv.ParseUint(it.Fee, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid fee"})
		return
	}

	if uintFee < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid value"})
		return
	}

	IntVal := strconv.Itoa(int(uintValue))
	val, ok := new(big.Int).SetString(IntVal, 10)
	if !ok {
		//log.Panic("new int failed\n")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid value"})
		return
	}

	IntFee := strconv.Itoa(int(uintFee))
	Fee, ok := new(big.Int).SetString(IntFee, 10)
	if !ok {
		//log.Panic("new int failed\n")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid fee"})
		return
	}

	if it.From == it.To {
		c.JSON(http.StatusBadRequest, gin.H{"error": "To address cannot be the same address"})
		return
	}

	Clt := c.MustGet("clt").(*client.Client)
	Clt.AccountStateMapLock.Lock()
	delete(Clt.AccountStateRequestMap, it.From)
	Clt.AccountStateMapLock.Unlock()
	Clt.SendAccountStateRequest2Worker(it.From)
	//ret := "Try for 10s... Cannot access blockEmulator Network."
	beginRequestTime := time.Now()
	var balance *big.Int
	for time.Since(beginRequestTime) < time.Second*10 {
		Clt.AccountStateMapLock.Lock()
		if state, ok := Clt.AccountStateRequestMap[it.From]; ok {
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

	if balance.Cmp(new(big.Int).Add(val, Fee)) < 0 {
		c.JSON(http.StatusOK, gin.H{"error": "Your balance is insufficient to send this transaction"})
		return
	}

	LastInvokeTimeMutex.Lock()
	if lastInvokeTime, exist := LastInvokeTime[it.From]; exist {
		if time.Since(lastInvokeTime) < 5*time.Second {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Send tx request too frequent! Please wait a moment !"})
			LastInvokeTimeMutex.Unlock()
			return
		}
		LastInvokeTime[it.From] = time.Now()
	} else {
		LastInvokeTime[it.From] = time.Now()
	}
	LastInvokeTimeMutex.Unlock()

	mytool.Mutex2.Lock()
	nonce := mytool.Nonce
	mytool.Nonce++
	mytool.Mutex2.Unlock()

	tx := core.NewTransaction(it.From, it.To, val, nonce, Fee)
	//tx.Isimportant = true

	mytool.Mutex1.Lock()
	mytool.UserRequestB2EQueue = append(mytool.UserRequestB2EQueue, tx)
	mytool.Mutex1.Unlock()

	// 返回成功响应
	data := map[string]interface{}{
		"message": "success",
		// 其他数据字段
	}
	c.JSON(http.StatusOK, data)
}
