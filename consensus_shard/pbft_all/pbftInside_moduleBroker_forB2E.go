// addtional module for new consensus
package pbft_all

import (
	"blockEmulator/core"
	"blockEmulator/message"
	"blockEmulator/networks"
	"blockEmulator/params"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// simple implementation of pbftHandleModule interface ...
// only for block request
type RawBrokerPbftExtraHandleMod_forB2E struct {
	pbftNode *PbftConsensusNode
}

// propose request with different types
func (rbhm *RawBrokerPbftExtraHandleMod_forB2E) HandleinPropose() (bool, *message.Request) {
	// new blocks
	block := rbhm.pbftNode.CurChain.GenerateBlock()
	r := &message.Request{
		RequestType: message.BlockRequest,
		ReqTime:     time.Now(),
	}
	r.Msg.Content = block.Encode()

	return true, r
}

// the diy operation in preprepare
func (rbhm *RawBrokerPbftExtraHandleMod_forB2E) HandleinPrePrepare(ppmsg *message.PrePrepare) bool {
	rbhm.pbftNode.requestPool[string(ppmsg.Digest)] = ppmsg.RequestMsg
	return true
}

// the operation in prepare, and in pbft + tx relaying, this function does not need to do any.
func (rbhm *RawBrokerPbftExtraHandleMod_forB2E) HandleinPrepare(pmsg *message.Prepare) bool {
	//fmt.Println("No operations are performed in Extra handle mod")
	return true
}

// the operation in commit.
func (rbhm *RawBrokerPbftExtraHandleMod_forB2E) HandleinCommit(cmsg *message.Commit) bool {
	r := rbhm.pbftNode.requestPool[string(cmsg.Digest)]
	// requestType ...
	block := core.DecodeB(r.Msg.Content)
	//rbhm.pbftNode.pl.Plog.Printf("S%dN%d : adding the block %d...now height = %d \n", rbhm.pbftNode.ShardID, rbhm.pbftNode.NodeID, block.Header.Number, rbhm.pbftNode.CurChain.CurrentBlock.Header.Number)
	rbhm.pbftNode.CurChain.AddBlock(block)
	//rbhm.pbftNode.pl.Plog.Printf("S%dN%d : added the block %d... \n", rbhm.pbftNode.ShardID, rbhm.pbftNode.NodeID, block.Header.Number)
	//rbhm.pbftNode.CurChain.PrintBlockChain()

	// now try to relay txs to other shards (for main nodes)
	if rbhm.pbftNode.NodeID == rbhm.pbftNode.view {
		// do normal operations for block
		//rbhm.pbftNode.pl.Plog.Printf("S%dN%d : main node is trying to send relay txs at height = %d \n", rbhm.pbftNode.ShardID, rbhm.pbftNode.NodeID, block.Header.Number)
		// generate brokertxs and collect txs excuted
		txExcuted := make([]*core.Transaction, 0)
		broker1Txs := make([]*core.Transaction, 0)
		broker2Txs := make([]*core.Transaction, 0)
		allocatedTxs := make([]*core.Transaction, 0)

		// generate block infos
		for _, tx := range block.Body {
			if tx.IsAllocatedRecipent || tx.IsAllocatedSender {
				allocatedTxs = append(allocatedTxs, tx)
				continue
			}
			//isInnerShardTx := tx.RawTxHash == nil
			//isBroker1Tx := !isInnerShardTx && tx.Sender == tx.OriginalSender
			isBroker1Tx := tx.Isbrokertx1

			//isBroker2Tx := !isInnerShardTx && tx.Recipient == tx.FinalRecipient
			isBroker2Tx := tx.Isbrokertx2
			if isBroker2Tx {
				broker2Txs = append(broker2Txs, tx)
			} else if isBroker1Tx {
				broker1Txs = append(broker1Txs, tx)
			} else {
				txExcuted = append(txExcuted, tx)
			}
		}
		// send seqID
		for sid := uint64(0); sid < rbhm.pbftNode.pbftChainConfig.ShardNums; sid++ {
			if sid == rbhm.pbftNode.ShardID {
				continue
			}
			sii := message.SeqIDinfo{
				SenderShardID: rbhm.pbftNode.ShardID,
				SenderSeq:     rbhm.pbftNode.sequenceID,
			}
			sByte, err := json.Marshal(sii)
			if err != nil {
				log.Panic()
			}
			msg_send := message.MergeMessage(message.CSeqIDinfo, sByte)
			go networks.TcpDial(msg_send, rbhm.pbftNode.ip_nodeTable[sid][0])
			//rbhm.pbftNode.pl.Plog.Printf("S%dN%d : sended sequence ids to %d\n", rbhm.pbftNode.ShardID, rbhm.pbftNode.NodeID, sid)
		}
		// send txs excuted in this block to the listener
		// add more message to measure more metrics
		bim := message.BlockInfoMsg{
			BlockBodyLength: len(block.Body),
			ExcutedTxs:      txExcuted,
			Broker1TxNum:    uint64(len(broker1Txs)),
			Broker1Txs:      broker1Txs,
			Broker2TxNum:    uint64(len(broker2Txs)),
			Broker2Txs:      broker2Txs,
			AllocatedTxs:    allocatedTxs,
			Epoch:           0,
			SenderShardID:   rbhm.pbftNode.ShardID,
			ProposeTime:     r.ReqTime,
			CommitTime:      time.Now(),
		}
		bByte, err := json.Marshal(bim)
		if err != nil {
			log.Panic()
		}
		msg_send := message.MergeMessage(message.CBlockInfo, bByte)
		go networks.TcpDial(msg_send, rbhm.pbftNode.ip_nodeTable[params.DeciderShard][0])
	}
	return true
}

func (rbhm *RawBrokerPbftExtraHandleMod_forB2E) HandleReqestforOldSeq(*message.RequestOldMessage) bool {
	fmt.Println("No operations are performed in Extra handle mod")
	return true
}

// the operation for sequential requests
func (rbhm *RawBrokerPbftExtraHandleMod_forB2E) HandleforSequentialRequest(som *message.SendOldMessage) bool {
	if int(som.SeqStartHeight-som.SeqEndHeight) != len(som.OldRequest) {
		rbhm.pbftNode.pl.Plog.Printf("S%dN%d : the SendOldMessage message is not enough\n", rbhm.pbftNode.ShardID, rbhm.pbftNode.NodeID)
	} else { // add the block into the node pbft blockchain
		for height := som.SeqStartHeight; height <= som.SeqEndHeight; height++ {
			r := som.OldRequest[height-som.SeqStartHeight]
			if r.RequestType == message.BlockRequest {
				b := core.DecodeB(r.Msg.Content)
				rbhm.pbftNode.CurChain.AddBlock(b)
			}
		}
		rbhm.pbftNode.sequenceID = som.SeqEndHeight + 1
		rbhm.pbftNode.CurChain.PrintBlockChain()
	}
	return true
}
