package client

import (
	"blockEmulator/client/client_log"
	"blockEmulator/core"
	"blockEmulator/message"
	"blockEmulator/networks"
	"blockEmulator/params"
	"blockEmulator/utils"
	"encoding/json"
	"log"
	"net"
	"sync"
)

// "Client" is a light-weight client, like a wallet.
type Client struct {
	IpAddr        string
	addr2ShardMap map[string]uint64
	clog          client_log.ClientLog
	// This map is used to store the AccountState
	AccountStateRequestMap map[string]*core.AccountState
	AccountStateMapLock    sync.Mutex

	// transaction Nonce
	nonceCounter uint64

	// tcplisten about
	tcpln       net.Listener
	tcpPoolLock sync.Mutex
	tcpStop     bool
}

func NewClient(_ip string) *Client {
	return &Client{
		IpAddr:                 _ip,
		addr2ShardMap:          make(map[string]uint64),
		AccountStateRequestMap: make(map[string]*core.AccountState),
		clog:                   *client_log.NewClientLog(),
		tcpStop:                false,
		nonceCounter:           1,
	}
}

func (c *Client) GetAddr2ShardMap(key string) uint64 {
	if val, ok := c.addr2ShardMap[key]; ok {
		return val
	}
	return uint64(utils.Addr2Shard(key))
}

func (c *Client) SetAddr2ShardMap(key string, val uint64) {
	c.addr2ShardMap[key] = val
}

// Client send a request to workers.
// This request wants to get a proof for a certain transaction.
//func (c *Client) SendTxProofRequest2Worker(tx *core.Transaction) {
//	// judge the shard
//	sid := c.GetAddr2ShardMap(tx.Recipient)
//
//	// encode message
//	crtxp := &message.ClientRequestTxProof{
//		TxHash:   tx.TxHash,
//		ClientIp: c.IpAddr,
//	}
//	crtxpByte, err := json.Marshal(crtxp)
//	if err != nil {
//		log.Panic(err)
//	}
//	send_msg := message.MergeMessage(message.CClientRequestTxProof, crtxpByte)
//
//	// send message
//	networks.TcpDial(send_msg, params.IPmap_nodeTable[sid][0])
//
//	c.clog.Clog.Printf("This tx proof request is sent")
//}

// send a account state request to worker
func (c *Client) SendAccountStateRequest2Worker(accountAddr string) {
	// judge the shard
	sid := c.GetAddr2ShardMap(accountAddr)

	// encode message
	cras := &message.ClientRequestAccountState{
		AccountAddr: accountAddr,
		ClientIp:    c.IpAddr,
	}
	crasByte, err := json.Marshal(cras)
	if err != nil {
		log.Panic(err)
	}
	send_msg := message.MergeMessage(message.CClientRequestAccountState, crasByte)

	// send message
	networks.TcpDial(send_msg, params.IPmap_nodeTable[sid][0])

	c.clog.Clog.Printf("This account state request is sent")
}
