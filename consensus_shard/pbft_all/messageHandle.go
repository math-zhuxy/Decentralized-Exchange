package pbft_all

import (
	"blockEmulator/message"
	"blockEmulator/networks"
	"blockEmulator/shard"
	"encoding/json"
	"log"
	"time"
)

// this func is only invoked by main node
func (p *PbftConsensusNode) Propose() {
	if p.view != p.NodeID {
		return
	}
	for {
		select {
		case <-p.pStop:
			p.pl.Plog.Printf("S%dN%d stop...\n", p.ShardID, p.NodeID)
			return
		default:
		}
		time.Sleep(time.Duration(int64(p.pbftChainConfig.BlockInterval)) * time.Millisecond)

		p.sequenceLock.Lock()
		//p.pl.Plog.Printf("S%dN%d get sequenceLock locked, now trying to propose...\n", p.ShardID, p.NodeID)
		// propose
		// implement interface to generate propose
		_, r := p.ihm.HandleinPropose()

		digest := getDigest(r)
		p.requestPool[string(digest)] = r
		//p.pl.Plog.Printf("S%dN%d put the request into the pool ...\n", p.ShardID, p.NodeID)

		ppmsg := message.PrePrepare{
			RequestMsg: r,
			Digest:     digest,
			SeqID:      p.sequenceID,
		}
		p.height2Digest[p.sequenceID] = string(digest)
		// marshal and broadcast
		ppbyte, err := json.Marshal(ppmsg)
		if err != nil {
			log.Panic()
		}
		msg_send := message.MergeMessage(message.CPrePrepare, ppbyte)
		networks.Broadcast(p.RunningNode.IPaddr, p.getNeighborNodes(), msg_send)
		p.pbftStage.Store(2)
	}
}

func (p *PbftConsensusNode) handlePrePrepare(content []byte) {
	//p.RunningNode.PrintNode()
	//fmt.Println("received the PrePrepare ...")

	// decode the message
	ppmsg := new(message.PrePrepare)
	err := json.Unmarshal(content, ppmsg)
	if err != nil {
		log.Panic(err)
	}
	flag := false

	p.pbftLock.Lock()
	defer p.pbftLock.Unlock()
	for p.pbftStage.Load() < 1 && ppmsg.SeqID >= p.sequenceID {
		p.conditionalVarpbftLock.Wait()
	}
	defer p.conditionalVarpbftLock.Broadcast()

	if digest := getDigest(ppmsg.RequestMsg); string(digest) != string(ppmsg.Digest) {
		p.pl.Plog.Printf("S%dN%d : the digest is not consistent, so refuse to prepare. \n", p.ShardID, p.NodeID)
	} else if p.sequenceID < ppmsg.SeqID {
		p.requestPool[string(getDigest(ppmsg.RequestMsg))] = ppmsg.RequestMsg
		p.height2Digest[ppmsg.SeqID] = string(getDigest(ppmsg.RequestMsg))
		p.pl.Plog.Printf("S%dN%d : the Sequence id is not consistent, so refuse to prepare. \n", p.ShardID, p.NodeID)
	} else {
		// do your operation in this interface
		flag = p.ihm.HandleinPrePrepare(ppmsg)
		p.requestPool[string(getDigest(ppmsg.RequestMsg))] = ppmsg.RequestMsg
		p.height2Digest[ppmsg.SeqID] = string(getDigest(ppmsg.RequestMsg))
	}
	// if the message is true, broadcast the prepare message
	flag = true
	if flag {
		pre := message.Prepare{
			Digest:     ppmsg.Digest,
			SeqID:      ppmsg.SeqID,
			SenderNode: p.RunningNode,
		}
		prepareByte, err := json.Marshal(pre)
		if err != nil {
			log.Panic()
		}
		// broadcast
		msg_send := message.MergeMessage(message.CPrepare, prepareByte)
		networks.Broadcast(p.RunningNode.IPaddr, p.getNeighborNodes(), msg_send)
		//p.pl.Plog.Printf("S%dN%d : has broadcast the prepare message \n", p.ShardID, p.NodeID)
		p.pbftStage.Add(1)
	}
}

func (p *PbftConsensusNode) handlePrepare(content []byte) {
	//p.pl.Plog.Printf("S%dN%d : received the Prepare ...\n", p.ShardID, p.NodeID)
	// decode the message
	pmsg := new(message.Prepare)
	err := json.Unmarshal(content, pmsg)
	if err != nil {
		log.Panic(err)
	}

	p.pbftLock.Lock()
	defer p.pbftLock.Unlock()
	for p.pbftStage.Load() < 2 && pmsg.SeqID >= p.sequenceID {
		p.conditionalVarpbftLock.Wait()
	}
	defer p.conditionalVarpbftLock.Broadcast()

	// if this message is out of date, return.
	if pmsg.SeqID < p.sequenceID {
		return
	}

	if _, ok := p.requestPool[string(pmsg.Digest)]; !ok {
		p.pl.Plog.Printf("S%dN%d : doesn't have the digest in the requst pool, refuse to commit\n", p.ShardID, p.NodeID)
	} else if p.sequenceID < pmsg.SeqID {
		p.pl.Plog.Printf("S%dN%d : inconsistent sequence ID, refuse to commit\n", p.ShardID, p.NodeID)
	} else {
		// if needed more operations, implement interfaces
		p.ihm.HandleinPrepare(pmsg)

		p.set2DMap(true, string(pmsg.Digest), pmsg.SenderNode)
		cnt := 0
		for range p.cntPrepareConfirm[string(pmsg.Digest)] {
			cnt++
		}
		// the main node will not send the prepare message
		specifiedcnt := int(2 * p.malicious_nums)
		if p.NodeID != p.view {
			specifiedcnt -= 1
		}

		// if the node has received 2f messages (itself included), and it haven't committed, then it commit
		p.lock.Lock()
		defer p.lock.Unlock()
		if cnt >= specifiedcnt && !p.isCommitBordcast[string(pmsg.Digest)] {
			//p.pl.Plog.Printf("S%dN%d : is going to commit\n", p.ShardID, p.NodeID)
			// generate commit and broadcast
			c := message.Commit{
				Digest:     pmsg.Digest,
				SeqID:      pmsg.SeqID,
				SenderNode: p.RunningNode,
			}
			commitByte, err := json.Marshal(c)
			if err != nil {
				log.Panic()
			}
			msg_send := message.MergeMessage(message.CCommit, commitByte)
			networks.Broadcast(p.RunningNode.IPaddr, p.getNeighborNodes(), msg_send)
			p.isCommitBordcast[string(pmsg.Digest)] = true
			//p.pl.Plog.Printf("S%dN%d : commit is broadcast\n", p.ShardID, p.NodeID)
			p.pbftStage.Add(1)
		}
	}
}

var flag bool = false

func (p *PbftConsensusNode) handleCommit(content []byte) {
	// decode the message
	cmsg := new(message.Commit)
	err := json.Unmarshal(content, cmsg)
	if err != nil {
		log.Panic(err)
	}

	p.pbftLock.Lock()
	defer p.pbftLock.Unlock()
	for p.pbftStage.Load() < 3 && cmsg.SeqID >= p.sequenceID {
		p.conditionalVarpbftLock.Wait()
	}
	defer p.conditionalVarpbftLock.Broadcast()

	//p.pl.Plog.Printf("S%dN%d received the Commit from ...%d\n", p.ShardID, p.NodeID, cmsg.SenderNode.NodeID)
	p.set2DMap(false, string(cmsg.Digest), cmsg.SenderNode)
	cnt := 0
	for range p.cntCommitConfirm[string(cmsg.Digest)] {
		cnt++
	}

	p.lock.Lock()
	defer p.lock.Unlock()
	// the main node will not send the prepare message
	required_cnt := int(2 * p.malicious_nums)
	if cnt >= required_cnt && !p.isReply[string(cmsg.Digest)] {
		//p.pl.Plog.Printf("S%dN%d : has received 2f + 1 commits ... \n", p.ShardID, p.NodeID)
		// if this node is left behind, so it need to requst blocks
		if _, ok := p.requestPool[string(cmsg.Digest)]; !ok {
			p.isReply[string(cmsg.Digest)] = true
			p.askForLock.Lock()
			// request the block
			sn := &shard.Node{
				NodeID:  p.view,
				ShardID: p.ShardID,
				IPaddr:  p.ip_nodeTable[p.ShardID][p.view],
			}
			orequest := message.RequestOldMessage{
				SeqStartHeight: p.sequenceID + 1,
				SeqEndHeight:   cmsg.SeqID,
				ServerNode:     sn,
				SenderNode:     p.RunningNode,
			}
			bromyte, err := json.Marshal(orequest)
			if err != nil {
				log.Panic()
			}

			p.pl.Plog.Printf("S%dN%d : is now requesting message (seq %d to %d) ... \n", p.ShardID, p.NodeID, orequest.SeqStartHeight, orequest.SeqEndHeight)
			msg_send := message.MergeMessage(message.CRequestOldrequest, bromyte)
			networks.TcpDial(msg_send, orequest.ServerNode.IPaddr)
		} else {
			// implement interface
			p.ihm.HandleinCommit(cmsg)
			p.isReply[string(cmsg.Digest)] = true
			p.pl.Plog.Printf("S%dN%d: this round of pbft %d is end \n", p.ShardID, p.NodeID, p.sequenceID)
			p.sequenceID += 1

			//发放出块奖励
			//if params.NodeID == p.view {
			//	bonus := 100
			//	if !flag {
			//		bonus = -999900
			//		flag = true
			//	}
			//
			//	for i := uint(0); i < uint(params.NodesInShard); i++ {
			//		if i == uint(params.NodeID) {
			//			sid := p.CurChain.Get_PartitionMap(global.NodeAccount)
			//			tx := core.NewTransaction(supervisor.Broker2EarnAddr, global.NodeAccount, big.NewInt(int64(bonus)), 1, big.NewInt(1))
			//			txs := make([]*core.Transaction, 0)
			//			txs = append(txs, tx)
			//			it := message.InjectTxs{
			//				Txs:       txs,
			//				ToShardID: sid,
			//			}
			//			itByte, err := json.Marshal(it)
			//			if err != nil {
			//				log.Panic(err)
			//			}
			//			send_msg := message.MergeMessage(message.CInject, itByte)
			//			go networks.TcpDial(send_msg, p.ip_nodeTable[sid][0])
			//		} else {
			//			global.NodeAccountMapLock.Lock()
			//			if _, ok := global.NodeAccountMap[i]; !ok {
			//				global.NodeAccountMapLock.Unlock()
			//				msg := message.QueryNodeAccount{FromNodeID: params.NodeID}
			//				Byte, _ := json.Marshal(msg)
			//				sendmsg := message.MergeMessage(message.CQueryNodeAccount, Byte)
			//				go networks.TcpDial(sendmsg, p.ip_nodeTable[params.ShardID][uint64(i)])
			//				<-global.NodeAccChan
			//			} else {
			//				global.NodeAccountMapLock.Unlock()
			//			}
			//
			//			global.NodeAccountMapLock.Lock()
			//			acc := global.NodeAccountMap[i]
			//			global.NodeAccountMapLock.Unlock()
			//			sid := p.CurChain.Get_PartitionMap(acc)
			//
			//			tx := core.NewTransaction(supervisor.Broker2EarnAddr, global.NodeAccountMap[i], big.NewInt(int64(bonus)), 1, big.NewInt(1))
			//			txs := make([]*core.Transaction, 0)
			//			txs = append(txs, tx)
			//			it := message.InjectTxs{
			//				Txs:       txs,
			//				ToShardID: sid,
			//			}
			//			itByte, err := json.Marshal(it)
			//			if err != nil {
			//				log.Panic(err)
			//			}
			//			send_msg := message.MergeMessage(message.CInject, itByte)
			//			go networks.TcpDial(send_msg, p.ip_nodeTable[sid][0])
			//		}
			//	}
			//}

			//从消息池删除本轮pbft的消息，避免内存泄漏
			delete(p.requestPool, string(cmsg.Digest))

			//delete(p.isReply, string(cmsg.Digest))
		}

		//查询本节点获得的奖励
		//m := new(message.QueryAccount)
		//m.FromNodeID = params.NodeID
		//m.FromShardID = params.ShardID
		//m.Account = global.NodeAccount
		//b, _ := json.Marshal(m)
		//ms := message.MergeMessage(message.CQueryAccount, b)
		//go networks.TcpDial(ms, p.ip_nodeTable[p.CurChain.Get_PartitionMap(global.NodeAccount)][0])

		p.pbftStage.Store(1)

		// if this node is a main node, then unlock the sequencelock
		if p.NodeID == p.view {
			p.sequenceLock.Unlock()
			//p.pl.Plog.Printf("S%dN%d get sequenceLock unlocked...\n", p.ShardID, p.NodeID)
		}
	}
}

// this func is only invoked by the main node,
// if the request is correct, the main node will send
// block back to the message sender.
// now this function can send both block and partition
func (p *PbftConsensusNode) handleRequestOldSeq(content []byte) {
	if p.view != p.NodeID {
		content = make([]byte, 0)
		return
	}

	rom := new(message.RequestOldMessage)
	err := json.Unmarshal(content, rom)
	if err != nil {
		log.Panic()
	}
	p.pl.Plog.Printf("S%dN%d : received the old message requst from ...", p.ShardID, p.NodeID)
	rom.SenderNode.PrintNode()

	oldR := make([]*message.Request, 0)
	for height := rom.SeqStartHeight; height <= rom.SeqEndHeight; height++ {
		if _, ok := p.height2Digest[height]; !ok {
			p.pl.Plog.Printf("S%dN%d : has no this digest to this height %d\n", p.ShardID, p.NodeID, height)
			break
		}
		if r, ok := p.requestPool[p.height2Digest[height]]; !ok {
			p.pl.Plog.Printf("S%dN%d : has no this message to this digest %d\n", p.ShardID, p.NodeID, height)
			break
		} else {
			oldR = append(oldR, r)
		}
	}
	p.pl.Plog.Printf("S%dN%d : has generated the message to be sent\n", p.ShardID, p.NodeID)

	p.ihm.HandleReqestforOldSeq(rom)

	// send the block back
	sb := message.SendOldMessage{
		SeqStartHeight: rom.SeqStartHeight,
		SeqEndHeight:   rom.SeqEndHeight,
		OldRequest:     oldR,
		SenderNode:     p.RunningNode,
	}
	sbByte, err := json.Marshal(sb)
	if err != nil {
		log.Panic()
	}
	msg_send := message.MergeMessage(message.CSendOldrequest, sbByte)
	networks.TcpDial(msg_send, rom.SenderNode.IPaddr)
	p.pl.Plog.Printf("S%dN%d : send blocks\n", p.ShardID, p.NodeID)
}

// node requst blocks and receive blocks from the main node
func (p *PbftConsensusNode) handleSendOldSeq(content []byte) {
	som := new(message.SendOldMessage)
	err := json.Unmarshal(content, som)
	if err != nil {
		log.Panic()
	}
	p.pl.Plog.Printf("S%dN%d : has received the SendOldMessage message\n", p.ShardID, p.NodeID)

	// implement interface for new consensus
	p.ihm.HandleforSequentialRequest(som)
	beginSeq := som.SeqStartHeight
	for idx, r := range som.OldRequest {
		p.requestPool[string(getDigest(r))] = r
		p.height2Digest[uint64(idx)+beginSeq] = string(getDigest(r))
		p.isReply[string(getDigest(r))] = true
		p.pl.Plog.Printf("this round of pbft %d is end \n", uint64(idx)+beginSeq)
	}
	p.sequenceID = som.SeqEndHeight + 1
	if rDigest, ok1 := p.height2Digest[p.sequenceID]; ok1 {
		if r, ok2 := p.requestPool[rDigest]; ok2 {
			ppmsg := &message.PrePrepare{
				RequestMsg: r,
				SeqID:      p.sequenceID,
				Digest:     getDigest(r),
			}
			flag := false
			flag = p.ihm.HandleinPrePrepare(ppmsg)
			if flag {
				pre := message.Prepare{
					Digest:     ppmsg.Digest,
					SeqID:      ppmsg.SeqID,
					SenderNode: p.RunningNode,
				}
				prepareByte, err := json.Marshal(pre)
				if err != nil {
					log.Panic()
				}
				// broadcast
				msg_send := message.MergeMessage(message.CPrepare, prepareByte)
				networks.Broadcast(p.RunningNode.IPaddr, p.getNeighborNodes(), msg_send)
				p.pl.Plog.Printf("S%dN%d : has broadcast the prepare message \n", p.ShardID, p.NodeID)
			}
		}
	}

	p.askForLock.Unlock()
}
