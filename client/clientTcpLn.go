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

// handle account states
func (c *Client) handleAccountStateMsg(content []byte) {
	asc := new(message.AccountStateForClient)
	err := json.Unmarshal(content, asc)
	if err != nil {
		log.Panic()
	}

	c.AccountStateMapLock.Lock()
	c.AccountStateRequestMap[asc.AccountAddr] = asc.AccountState
	c.AccountStateMapLock.Unlock()
}
