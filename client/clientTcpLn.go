// client listen message from tcp conn
package client

import (
	"blockEmulator/message"
	"bufio"
	"encoding/json"
	"io"
	"log"
	"net"
)

// handle the raw message, send it to corresponded interfaces
func (c *Client) handleMessageByType(msg []byte) {
	msgType, content := message.SplitMessage(msg)
	switch msgType {
	case message.CAccountStateForClient:
		c.handleAccountStateMsg(content)
	case message.CAccountTransactionForClient:
		c.handleTransactionMsg(content)

	default:
		return
	}
}

func (c *Client) handleMsg(con net.Conn) {
	defer con.Close()
	clientReader := bufio.NewReader(con)
	for {
		clientRequest, err := clientReader.ReadBytes('\n')
		if c.tcpStop {
			return
		}
		switch err {
		case nil:
			c.tcpPoolLock.Lock()
			c.handleMessageByType(clientRequest)
			c.tcpPoolLock.Unlock()
		case io.EOF:
			log.Println("client closed the connection by terminating the process")
			return
		default:
			log.Printf("error: %v\n", err)
			return
		}
	}
}

// client listen messages by tcp
func (c *Client) ClientListen() error {
	ln, err := net.Listen("tcp", c.IpAddr)
	c.tcpln = ln
	if err != nil {
		log.Panic(err)
	}
	for {
		conn, err := c.tcpln.Accept()
		if err != nil {
			return err
		}
		go c.handleMsg(conn)
	}
}

// detail operations for each message
//func (c *Client) handleTxOnChainProof(content []byte) {
//	txop := new(message.TxOnChainProof)
//	err := json.Unmarshal(content, txop)
//	if err != nil {
//		log.Panic()
//	}
//	c.clog.Clog.Printf("TxOnChainProof message from %d is received \n", txop.ShardId)
//
//	if !txop.IsExist {
//		vals := []interface{}{
//			"No data about Tx",
//			[]byte(txop.Txhash),
//			"is in Shard",
//			txop.ShardId,
//			"now.",
//		}
//		c.clog.Clog.Printf("%v\n\n", vals)
//		return
//	}
//
//	// check the proof
//	proof := rawdb.NewMemoryDatabase()
//	listLen := len(txop.KeyList)
//	c.clog.Clog.Printf("The length of proof is %d\n", listLen)
//	for i := 0; i < listLen; i++ {
//		proof.Put(txop.KeyList[i], txop.ValueList[i])
//	}
//	if _, err := trie.VerifyProof(common.BytesToHash(txop.TxRoot), []byte(txop.Txhash), proof); err != nil {
//		c.clog.Clog.Printf("This proof is err!\n\n")
//		return
//	}
//
//	// if proof is right then
//	vals := []interface{}{
//		"Tx proof of",
//		[]byte(txop.Txhash),
//		"in block & TxRoot",
//		[]byte(txop.BlockHash),
//		[]byte(txop.TxRoot),
//		"with height",
//		txop.BlockHeight,
//		"is verified.",
//	}
//	c.clog.Clog.Printf("%v\n\n", vals)
//}

// handle account states
func (c *Client) handleAccountStateMsg(content []byte) {
	asc := new(message.AccountStateForClient)
	err := json.Unmarshal(content, asc)
	if err != nil {
		log.Panic()
	}
	//vals := []interface{}{
	//	"Account State of ",
	//	"0x" + asc.AccountAddr,
	//	"is received in block",
	//	[]byte(asc.BlockHash),
	//	"with height",
	//	asc.BlockHeight,
	//	". Its balance is ",
	//	asc.AccountState.Balance,
	//}
	//c.clog.Clog.Printf("%v\n\n", vals)
	c.AccountStateMapLock.Lock()
	c.AccountStateRequestMap[asc.AccountAddr] = asc.AccountState
	c.AccountStateMapLock.Unlock()
}

func (c *Client) handleTransactionMsg(content []byte) {

}
