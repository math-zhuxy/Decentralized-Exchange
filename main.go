package main

import (
	"blockEmulator/build"
	"blockEmulator/vm/state"
	"encoding/gob"
	"fmt"
	"net/http"
	_ "net/http/pprof"

	"github.com/spf13/pflag"
)

var (
	shardNum int
	nodeNum  int
	shardID  int
	nodeID   int
	modID    int
	isClient bool
	isGen    bool
)

func main() {
	gob.Register(state.Wrapper{})
	gob.Register(state.CreateObjectChange{})
	gob.Register(state.CreateContractChange{})
	gob.Register(state.SelfDestructChange{})
	gob.Register(state.BalanceChange{})
	gob.Register(state.NonceChange{})
	gob.Register(state.StorageChange{})
	gob.Register(state.CodeChange{})
	gob.Register(state.RefundChange{})
	gob.Register(state.AddLogChange{})
	gob.Register(state.TouchChange{})
	gob.Register(state.AccessListAddAccountChange{})
	gob.Register(state.AccessListAddSlotChange{})
	gob.Register(state.TransientStorageChange{})

	pflag.IntVarP(&shardNum, "shardNum", "S", 2, "indicate that how many shards are deployed")
	pflag.IntVarP(&nodeNum, "nodeNum", "N", 4, "indicate how many nodes of each shard are deployed")
	pflag.IntVarP(&shardID, "shardID", "s", 0, "id of the shard to which this node belongs, for example, 0")
	pflag.IntVarP(&nodeID, "nodeID", "n", 0, "id of this node, for example, 0")
	pflag.IntVarP(&modID, "modID", "m", 4, "choice Committee Method,for example, 0, [CLPA_Broker,CLPA,Broker,Relay,Broker_b2e] ")
	pflag.BoolVarP(&isClient, "client", "c", false, "whether this node is a client")
	pflag.BoolVarP(&isGen, "gen", "g", false, "generation bat")
	pflag.Parse()

	//return
	if isGen {
		build.GenerateBatFile(nodeNum, shardNum, modID)
		build.GenerateShellFile(nodeNum, shardNum, modID)
		return
	}

	if isClient {
		go func() {
			if err := http.ListenAndServe(fmt.Sprintf(":%d", 11999), nil); err != nil {
				panic("pprof server start error: " + err.Error())
			}
		}()
		build.BuildSupervisor(uint64(nodeNum), uint64(shardNum), uint64(modID))
	} else {
		go func() {
			if err := http.ListenAndServe(fmt.Sprintf(":%d", 12000+shardID*10+nodeID), nil); err != nil {
				panic("pprof server start error: " + err.Error())
			}
		}()
		build.InitKey(shardID, nodeID)
		build.BuildNewPbftNode(uint64(nodeID), uint64(nodeNum), uint64(shardID), uint64(shardNum), uint64(modID))
	}
}
