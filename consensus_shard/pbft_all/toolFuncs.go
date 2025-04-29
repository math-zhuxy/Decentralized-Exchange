package pbft_all

import (
	"blockEmulator/message"
	"blockEmulator/shard"
	"crypto/sha256"
	"encoding/json"
	"log"
)

// set 2d map, only for pbft maps, if the first parameter is true, then set the cntPrepareConfirm map,
// otherwise, cntCommitConfirm map will be set
func (p *PbftConsensusNode) set2DMap(isPrePareConfirm bool, key string, val *shard.Node) {
	if isPrePareConfirm {
		if _, ok := p.cntPrepareConfirm[key]; !ok {
			p.cntPrepareConfirm[key] = make(map[*shard.Node]bool)
		}
		p.cntPrepareConfirm[key][val] = true
	} else {
		if _, ok := p.cntCommitConfirm[key]; !ok {
			p.cntCommitConfirm[key] = make(map[*shard.Node]bool)
		}
		p.cntCommitConfirm[key][val] = true
	}
}

// get neighbor nodes in a shard
func (p *PbftConsensusNode) getNeighborNodes() []string {
	receiverNodes := make([]string, 0)
	for _, ip := range p.ip_nodeTable[p.ShardID] {
		receiverNodes = append(receiverNodes, ip)
	}
	return receiverNodes
}

// get the digest of request
func getDigest(r *message.Request) []byte {
	b, err := json.Marshal(r)
	if err != nil {
		log.Panic(err)
	}
	hash := sha256.Sum256(b)
	return hash[:]
}
