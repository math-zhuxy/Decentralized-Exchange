package pbft_all

import (
	"blockEmulator/message"
	"blockEmulator/networks"
	"blockEmulator/params"
	"encoding/json"
	"log"
	"math/big"
)

// This module used in the blockChain using transaction relaying mechanism.
// "Raw" means that the pbft only make block consensus.
type RawBrokerOutsideModule_forB2E struct {
	pbftNode *PbftConsensusNode
}

// msgType canbe defined in message
func (rrom *RawBrokerOutsideModule_forB2E) HandleMessageOutsidePBFT(msgType message.MessageType, content []byte) bool {
	switch msgType {
	case message.CSeqIDinfo:
		rrom.handleSeqIDinfos(content)
	case message.CInject:
		rrom.handleInjectTx(content)
	case message.CInjectHead:
		rrom.handleInjectTxHead(content)
	case message.DAccountBalanceQuery:
		rrom.handleAccQuery(content)
	default:
	}
	return true
}

func (rrom *RawBrokerOutsideModule_forB2E) handleAccQuery(content []byte) {
	it := new(message.AccountBalanceQuery)
	err := json.Unmarshal(content, it)
	if err != nil {
		log.Panic(err)
	}
	balance := new(big.Int)
	if rrom.pbftNode.CurChain.BalanceMap[it.TXType][it.Addr] == nil {
		balance.Set(params.Init_Balance)
	} else {
		balance.Set(rrom.pbftNode.CurChain.BalanceMap[it.TXType][it.Addr])
	}
	msg := &message.AccountBalanceReply{
		Addr:    it.Addr,
		Balance: balance,
		TXType:  it.TXType,
	}
	msg_bytes, _ := json.Marshal(msg)
	final_msg := message.MergeMessage(message.DAccountBalanceReply, msg_bytes)
	go networks.TcpDial(final_msg, params.SupervisorAddr)
}

// receive SeqIDinfo
func (rrom *RawBrokerOutsideModule_forB2E) handleSeqIDinfos(content []byte) {
	sii := new(message.SeqIDinfo)
	err := json.Unmarshal(content, sii)
	if err != nil {
		log.Panic(err)
	}
	//rrom.pbftNode.pl.Plog.Printf("S%dN%d : has received SeqIDinfo from shard %d, the senderSeq is %d\n", rrom.pbftNode.ShardID, rrom.pbftNode.NodeID, sii.SenderShardID, sii.SenderSeq)
	rrom.pbftNode.seqMapLock.Lock()
	rrom.pbftNode.seqIDMap[sii.SenderShardID] = sii.SenderSeq
	rrom.pbftNode.seqMapLock.Unlock()
	//rrom.pbftNode.pl.Plog.Printf("S%dN%d : has handled SeqIDinfo msg\n", rrom.pbftNode.ShardID, rrom.pbftNode.NodeID)
}

func (rrom *RawBrokerOutsideModule_forB2E) handleInjectTx(content []byte) {
	it := new(message.InjectTxs)
	err := json.Unmarshal(content, it)
	if err != nil {
		log.Panic(err)
	}
	rrom.pbftNode.CurChain.Txpool.AddTxs2Pool(it.Txs)
	rrom.pbftNode.pl.Plog.Printf("S%dN%d : has handled injected txs msg, txs: %d \n", rrom.pbftNode.ShardID, rrom.pbftNode.NodeID, len(it.Txs))
}

func (rrom *RawBrokerOutsideModule_forB2E) handleInjectTxHead(content []byte) {
	it := new(message.InjectTxs)
	err := json.Unmarshal(content, it)
	if err != nil {
		log.Panic(err)
	}
	rrom.pbftNode.CurChain.Txpool.AddTxs2Pool_Head(it.Txs)
	rrom.pbftNode.pl.Plog.Printf("S%dN%d : has handled injected txs msg, txs: %d \n", rrom.pbftNode.ShardID, rrom.pbftNode.NodeID, len(it.Txs))
}
