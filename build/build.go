package build

import (
	"blockEmulator/consensus_shard/pbft_all"
	"blockEmulator/core"
	"blockEmulator/global"
	"blockEmulator/message"
	"blockEmulator/networks"
	"blockEmulator/params"
	"blockEmulator/supervisor"
	"blockEmulator/utils"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"golang.org/x/crypto/sha3"
)

func initConfig(nid, nnm, sid, snm uint64) *params.ChainConfig {
	params.ShardNum = int(snm)
	for i := uint64(0); i < snm; i++ {
		if _, ok := params.IPmap_nodeTable[i]; !ok {
			params.IPmap_nodeTable[i] = make(map[uint64]string)
		}
		for j := uint64(0); j < nnm; j++ {
			params.IPmap_nodeTable[i][j] = "127.0.0.1:" + strconv.Itoa(28800+int(i)*100+int(j))
		}
	}
	params.IPmap_nodeTable[params.DeciderShard] = make(map[uint64]string)
	params.IPmap_nodeTable[params.DeciderShard][0] = params.SupervisorAddr
	params.NodesInShard = int(nnm)
	params.ShardNum = int(snm)

	pcc := &params.ChainConfig{
		ChainID:        sid,
		NodeID:         nid,
		ShardID:        sid,
		Nodes_perShard: uint64(params.NodesInShard),
		ShardNums:      snm,
		BlockSize:      uint64(params.MaxBlockSize_global),
		BlockInterval:  uint64(params.Block_Interval),
		InjectSpeed:    uint64(params.InjectSpeed),
	}
	return pcc
}

func BuildSupervisor(nnm, snm, mod uint64) {
	var measureMod []string
	if mod == 0 || mod == 2 || mod == 4 {
		measureMod = params.MeasureBrokerMod
	} else {
		measureMod = params.MeasureRelayMod
	}

	lsn := new(supervisor.Supervisor)
	lsn.NewSupervisor(params.SupervisorAddr, initConfig(123, nnm, 123, snm), params.CommitteeMethod[mod], measureMod...)
	println("Model : ", params.CommitteeMethod[mod], " build sucess!")
	go lsn.TcpListen()
	time.Sleep(5000 * time.Millisecond)
	//lsn.CreateBrokerContract()
	time.Sleep(5000 * time.Millisecond)
	go lsn.SupervisorTxHandling()
	go lsn.RunHTTP()

	// 启动时编译合约并部署，避免每次都需要手动编译合约
	// 这里的合约编译和部署是为了在系统启动时预先部署一个 NFT 合约，对于dex来说目前无用，先放着
	go func() {
		time.Sleep(3 * time.Second)

		// macOS 路径，指向你的 NFT 合约文件（注意替换为你自己的路径）
		contractFileName := "./contract/NFT.sol"

		// 执行 solc 编译合约，输出 abi 和 bin 到同一目录
		err := exec.Command(
			"solc", contractFileName,
			"--abi", "--bin",
			"--optimize", "--overwrite", "-o", "./contract",
		).Run()
		if err != nil {
			log.Panic("执行 solc 出错:", err)
			return
		}

		// 读取编译出的 bin 文件
		data, err := os.ReadFile("./contract/NFT.bin")
		if err != nil {
			fmt.Println("读取 bin 文件出错:", err)
			return
		}

		strData := string(data)
		fmt.Println("合约 Bytecode 内容:", strData)

		byteData, err := hex.DecodeString(strData)
		if err != nil {
			log.Panic("十六进制解码出错:", err)
		}

		// 系统启动时预先通过这个地址部署合约
		from := "0000000000000000000000000000000000000001"
		Transaction := core.NewTransactionContract(from, "", big.NewInt(0), 1, byteData)
		Transaction.GasPrice = big.NewInt(1)
		Transaction.Gas = 100000000
		Transaction.UUID = "1234"

		var txs = []*core.Transaction{Transaction}
		it := message.InjectTxs{
			Txs:       txs,
			ToShardID: uint64(utils.Addr2Shard(from)),
		}
		itByte, err := json.Marshal(it)
		if err != nil {
			log.Panic("序列化交易失败:", err)
		}

		send_msg := message.MergeMessage(message.CInject, itByte)

		// 将交易发送到对应 shard 节点（ip 需保证 lsn.Ip_nodeTable 配置正确）
		go networks.TcpDial(send_msg, lsn.Ip_nodeTable[uint64(utils.Addr2Shard(from))][0])
	}()

	global.ShardID = params.DeciderShard
	global.NodeID = 0

	for {
		time.Sleep(time.Second)
	}
}

func BuildNewPbftNode(nid, nnm, sid, snm, mod uint64) {
	worker := pbft_all.NewPbftNode(sid, nid, initConfig(nid, nnm, sid, snm), params.CommitteeMethod[mod])
	if nid == 0 {
		go worker.Propose()
		worker.TcpListen()
	} else {
		worker.TcpListen()
	}
}

func InitKey(sid int, nid int) {
	keypath := params.KeyWrite_path
	filePath1 := filepath.Join(keypath, fmt.Sprintf("publickeyS%dN%d", sid, nid))
	filePath2 := filepath.Join(keypath, fmt.Sprintf("privatekeyS%dN%d", sid, nid))
	if _, err := os.Stat(keypath); os.IsNotExist(err) {
		// 目录不存在，创建目录和文件
		if err := os.MkdirAll(keypath, 0755); err != nil {
			fmt.Printf("创建目录失败: %v\n", err)
			return
		}
	}

	_, err1 := os.Stat(filePath1)
	_, err2 := os.Stat(filePath2)
	if os.IsNotExist(err1) && os.IsNotExist(err2) {

		// 生成 RSA 密钥对
		//privateKey, err := rsa.GenerateKey(rand.Reader, 4096)

		privateKey, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
		if err != nil {
			log.Fatalf("生成 ECDSA 密钥对失败: %v", err)
		}
		// 将私钥转换为 ASN.1 PKCS#1 DER 编码
		//privDER := x509.MarshalPKCS1PrivateKey(privateKey)
		privDER, _ := x509.MarshalECPrivateKey(privateKey)
		// 将 DER 编码的私钥转换为 PEM 格式
		privPEM := pem.EncodeToMemory(&pem.Block{
			//Type:  "RSA PRIVATE KEY",
			Type:  "ECDSA PRIVATE KEY",
			Bytes: privDER,
		})
		// 将公钥提取为 *ecdsa.PublicKey 类型
		publicKey := &privateKey.PublicKey
		// 将公钥转换为 ASN.1 PKIX DER 编码
		pubDER, err := x509.MarshalPKIXPublicKey(publicKey)
		if err != nil {
			log.Fatalf("公钥编码失败: %v", err)
		}
		// 将 DER 编码的公钥转换为 PEM 格式
		pubPEM := pem.EncodeToMemory(&pem.Block{
			//Type:  "RSA PUBLIC KEY",
			Type:  "ECDSA PUBLIC KEY",
			Bytes: pubDER,
		})
		// 打印私钥和公钥
		fmt.Println("私钥:")
		fmt.Println(string(privPEM))
		fmt.Println("公钥:")
		fmt.Println(string(pubPEM))

		global.NodePublicKey = string(pubPEM)

		if err := ioutil.WriteFile(filePath1, []byte(string(pubPEM)), 0644); err != nil {
			fmt.Printf("写入文件失败: %v\n", err)
			return
		}
		if err := ioutil.WriteFile(filePath2, []byte(string(privPEM)), 0644); err != nil {
			fmt.Printf("写入文件失败: %v\n", err)
			return
		}

		hash := sha3.New256()
		hash.Write(pubDER)
		hashSum := hash.Sum(nil)

		// 截取最后的20字节作为账户地址
		accountAddress := hashSum[len(hashSum)-20:]

		fmt.Printf("Account address: %x\n", accountAddress)

		global.NodeAccount = fmt.Sprintf("%x", accountAddress)
		fmt.Println(global.NodeAccount)
	} else {
		if os.IsNotExist(err1) || os.IsNotExist(err2) {
			log.Panic("error")
			return
		}
		publicKey, err := ioutil.ReadFile(filePath1)
		if err != nil {
			fmt.Printf("读取文件失败: %v\n", err)
			return
		}
		fmt.Println("读取到已有公钥：", string(publicKey))

		//pubDER1, err := x509.MarshalPKIXPublicKey(publicKey)
		//if err != nil {
		//	log.Fatalf("公钥编码失败: %v", err)
		//}

		privatekey, err := ioutil.ReadFile(filePath2)
		if err != nil {
			fmt.Printf("读取文件失败: %v\n", err)
			return
		}
		fmt.Println("读取到已有私钥：", string(privatekey))

		block1, _ := pem.Decode([]byte(publicKey))
		//if block1 == nil || block1.Type != "RSA PUBLIC KEY" {
		if block1 == nil || block1.Type != "ECDSA PUBLIC KEY" {
			fmt.Println("failed to decode PEM block containing public key")
			return
		}

		block2, _ := pem.Decode([]byte(privatekey))
		//if block2 == nil || block2.Type != "RSA PRIVATE KEY" {
		if block2 == nil || block2.Type != "ECDSA PRIVATE KEY" {
			fmt.Println("failed to decode PEM block containing private key")
			return
		}

		//privateKey, err := x509.ParsePKCS1PrivateKey(block2.Bytes)
		privateKey, err := x509.ParseECPrivateKey(block2.Bytes)

		//privDER1 := x509.MarshalPKCS1PrivateKey(privateKey)
		privDER1, _ := x509.MarshalECPrivateKey(privateKey)
		// 将 DER 编码的私钥转换为 PEM 格式
		privPEM1 := pem.EncodeToMemory(&pem.Block{
			Type: "ECDSA PRIVATE KEY",
			//Type:  "RSA PRIVATE KEY",
			Bytes: privDER1,
		})
		// 将公钥提取为 *rsa.PublicKey 类型
		publicKey2 := &privateKey.PublicKey
		// 将公钥转换为 ASN.1 PKIX DER 编码
		pubDER2, err := x509.MarshalPKIXPublicKey(publicKey2)
		if err != nil {
			log.Fatalf("公钥编码失败: %v", err)
		}
		// 将 DER 编码的公钥转换为 PEM 格式
		pubPEM2 := pem.EncodeToMemory(&pem.Block{
			//Type:  "RSA PUBLIC KEY",
			Type:  "ECDSA PUBLIC KEY",
			Bytes: pubDER2,
		})

		if string(pubDER2) != string(block1.Bytes) {
			panic("公钥和私钥提供的公钥不匹配")
		}

		// 打印私钥和公钥
		fmt.Println("私钥:")
		fmt.Println(string(privPEM1))
		fmt.Println("公钥:")
		fmt.Println(string(pubPEM2))

		hash := sha3.New256()
		hash.Write(pubDER2)
		hashSum := hash.Sum(nil)

		// 截取最后的20字节作为账户地址
		accountAddress := hashSum[len(hashSum)-20:]

		fmt.Printf("Account address: %x\n", accountAddress)

		global.NodeAccount = fmt.Sprintf("%x", accountAddress)
		fmt.Println(global.NodeAccount)
	}

	global.NodeAccountMap = make(map[uint]string)
}

const (
	Port = 8082
)
