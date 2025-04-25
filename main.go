package main

import (
	"blockEmulator/build"
	"blockEmulator/vm/state"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/pflag"
	"net/http"
	_ "net/http/pprof"
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

type A interface {
	test() string
	//MarshalType() string
}

type B struct {
	Addr common.Address
	//Type string `json:"type"` // 类型标签
}

func (b *B) test() string {
	fmt.Println("b test")
	return "B test"
}

//func (b *B) MarshalType() string {
//	return "B"
//}

type C struct {
	Addr int
	//Type string `json:"type"` // 类型标签
}

func (c *C) test() string {
	fmt.Println("c test")
	return "C test"
}

//func (c *C) MarshalType() string {
//	return "C"
//}

type D struct {
	Aa   string
	List []Wrapper
}

type Wrapper struct {
	OriginalObj A               `json:"-"`    // 存储原始的对象
	Type        string          `json:"type"` //类型
	Data        json.RawMessage `json:"data"` // 存储序列化后的数据
}

func (w *Wrapper) UnmarshalJSON(data []byte) error {
	var aux struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	w.Type = aux.Type
	w.Data = aux.Data

	switch w.Type {
	case "B":
		w.OriginalObj = &B{}
	case "C":
		w.OriginalObj = &C{}
	default:
		return fmt.Errorf("unknown type: %s", w.Type)
	}
	return json.Unmarshal(w.Data, w.OriginalObj)
}

func (w *Wrapper) MarshalJSON() ([]byte, error) {
	aux := struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}{
		Type: w.Type,
	}
	var err error
	aux.Data, err = json.Marshal(w.OriginalObj)
	if err != nil {
		return nil, err
	}
	return json.Marshal(aux)
}

func test2() {
	a1 := B{Addr: common.HexToAddress("0x111")}
	a2 := C{Addr: 1234}
	var a3 []Wrapper
	a3 = append(a3, Wrapper{Type: "B", OriginalObj: &a1})
	a3 = append(a3, Wrapper{Type: "C", OriginalObj: &a2})

	m := &D{
		Aa:   "aaa",
		List: a3,
	}
	b, err := json.Marshal(m)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(b))

	m1 := &D{}
	err = json.Unmarshal(b, m1)
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, item := range m1.List {
		fmt.Printf("Type: %s, Value: %+v\n", item.Type, item.OriginalObj)
		item.OriginalObj.test()
	}
}

//type A interface {
//	test() string
//}
//
//type B struct {
//	Addr common.Address
//}
//
//func (b *B) test() string {
//	fmt.Println(b.Addr)
//	return "123"
//}
//
//type C struct {
//	Addr int
//}
//
//func (c *C) test() string {
//	fmt.Println(c.Addr)
//	return "456"
//}
//
//type D struct {
//	Aa   string
//	List []A
//}
//
//func test2() {
//	a1 := B{Addr: common.HexToAddress("0x111")}
//	a2 := C{Addr: 1234}
//	var a3 = make([]A, 0)
//	a3 = append(a3, &a1)
//	a3 = append(a3, &a2)
//
//	m := new(D)
//	m.Aa = "aaa"
//	m.List = a3
//	b, err := json.Marshal(m)
//	if err != nil {
//		fmt.Println(err)
//	}
//	fmt.Println(string(b))
//
//	m1 := new(D)
//	err = json.Unmarshal(b, m1)
//	if err != nil {
//		fmt.Println(err)
//	}
//
//	m1.List[0].test()
//	m1.List[1].test()
//}

func test() {
	a := B{Addr: common.HexToAddress("0x1")}
	b, err := json.Marshal(a)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(b))

	c := new(B)
	c.test()

	err = json.Unmarshal(b, &c)
	if err != nil {
		fmt.Println(err)
	}
	c.test()

}

//TODO 执行一个合约时分片内共用一个statedb
func main() {



	//fp := "./record/ldb"
	//db, _ := rawdb.NewLevelDBDatabase(fp, 0, 1, "accountState", false)
	//triedb1 := triedb.NewDatabase(db, &triedb.Config{
	//	//Cache:     0,
	//	Preimages: true,
	//	IsVerkle:  false,
	//})
	//statusTrie := trie.NewEmpty(triedb1)
	//
	//db1:=state.NewDatabase(triedb1, nil)
	//
	//
	//statedb1,_:=state.New(statusTrie.Hash(),db1)
	//statedb2,_:=state.New(statusTrie.Hash(), db1)
	//addr,_:=hex.DecodeString("a42af2c70d316684e57aefcc6e393fecb1c7e84e")
	//statedb1.AddBalance(common.Address(addr),uint256.NewInt(100),tracing.BalanceAddByXBZ)
	//
	//statedb2.AddBalance(common.Address(addr),uint256.NewInt(200),tracing.BalanceAddByXBZ)
	//
	//fmt.Println(statedb1.GetBalance(common.Address(addr)))
	//fmt.Println(statedb2.GetBalance(common.Address(addr)))
	//h2, er := statedb2.Commit(0,false)
	//if er != nil {
	//	fmt.Println("2:"+er.Error())
	//}
	//h1, err := statedb1.Commit(0,false)
	//if err != nil {
	//	fmt.Println("1:"+err.Error())
	//}
	//
	//
	//
	//statedb3,e1:=state.New(h1, db1)
	//statedb4,e2:=state.New(h2, db1)
	//if e1 !=nil{
	//	fmt.Println("3:"+e1.Error())
	//}
	//if e2 !=nil{
	//	fmt.Println("4:"+e2.Error())
	//}
	//fmt.Println(statedb3.GetBalance(common.Address(addr)))
	//fmt.Println(statedb4.GetBalance(common.Address(addr)))
	//
	//return

	//a,err:= hex.DecodeString("a42af2c70d316684e57aefcc6e393fecb1c7e84e")
	//if err != nil {fmt.Println(err)}
	//
	//b := vm.AccountRef(a[:])
	//
	//c,e:=json.Marshal(b)
	//if e != nil {fmt.Println(e)}
	//fmt.Println(string(c))
	//
	//d:=new(vm.AccountRef)
	//e1:=json.Unmarshal(c,d)
	//
	//if e1 != nil {fmt.Println(e1)}
	//
	//
	//fmt.Println(hex.EncodeToString(d.Address().Bytes()))

	//test2()
	//return

	//m := new(message.OpCallRes)
	//fmt.Println(m)

	//a:=common.Address{}
	//a = common.HexToAddress("0x123")
	//c,e:=json.Marshal(a)
	//
	//if e != nil {fmt.Println(e)}
	//fmt.Println(string(c))
	//
	//d:=common.Address{}
	//e1:=json.Unmarshal(c, &d)
	//
	//if e1 != nil {fmt.Println(e1)}
	//
	//
	//fmt.Println(d)
	//
	//return


	//fmt.Println(uuid.New().String())
	//return


	//0xd82cf790000000000000000000000000000000000000000000000000000000000000007b
	//s:="getA(uint256)"
	//0xb60d4288
	//s:="create()"
	//s1:=crypto.Keccak256Hash([]byte(s))
	//fmt.Println(hex.EncodeToString(s1[:])[0:8])
	//b,_:=hex.DecodeString(hex.EncodeToString(s1[:])[0:8])
	//fmt.Println(b)
	//fmt.Println(hex.EncodeToString(b))
	//fmt.Println(s1[0:16])
	////return
	//
	//
	//return
	//
	//params.ShardNum = 3
	//fmt.Println(uint64(utils.Addr2Shard("9fe093098da2eceb3bcd246a203008e060aa9392")))
	//fmt.Println(uint64(utils.Addr2Shard("5d3604a5794ad4492ccefcf9de6a93f56fc90092")))
	//
	//
	//s1,_:=hex.DecodeString("9fe093098da2eceb3bcd246a203008e060aa9392")
	//s11:=common.Address(s1)
	//s2,_:=hex.DecodeString("5d3604a5794ad4492ccefcf9de6a93f56fc9009d")
	//s22:=common.Address(s2)
	//fmt.Println(utils.Addr2Shard(hex.EncodeToString(s11[:])))
	//fmt.Println(utils.Addr2Shard(hex.EncodeToString(s22[:])))
	//
	//return

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
			if err := http.ListenAndServe(fmt.Sprintf(":%d",11999), nil); err != nil {
				panic("pprof server start error: " + err.Error())
			}
		}()
		build.BuildSupervisor(uint64(nodeNum), uint64(shardNum), uint64(modID))
	} else {
		go func() {
			if err := http.ListenAndServe(fmt.Sprintf(":%d",12000+shardID*10+nodeID), nil); err != nil {
				panic("pprof server start error: " + err.Error())
			}
		}()
		build.InitKey(shardID, nodeID)
		build.BuildNewPbftNode(uint64(nodeID), uint64(nodeNum), uint64(shardID), uint64(shardNum), uint64(modID))
	}
}
