package broker

import (
	"blockEmulator/message"
	"blockEmulator/params"
	"bufio"
	"fmt"
	"math/big"
	"os"
)

type ShardBalanceInt map[string]map[uint64]*big.Int
type ShardBalanceFloat map[string]map[uint64]*big.Float

type Broker struct {
	BrokerRawMegs  map[string]*message.BrokerRawMeg
	ChainConfig    *params.ChainConfig
	BrokerAddress  []string
	BrokerBalance  map[string]ShardBalanceInt
	LockBalance    map[string]ShardBalanceInt
	ProfitBalance  map[string]ShardBalanceFloat
	RawTx2BrokerTx map[string][]string
	Brokerage      *big.Float
}

func (b *Broker) NewBroker(pcc *params.ChainConfig) {
	b.BrokerRawMegs = make(map[string]*message.BrokerRawMeg)
	b.RawTx2BrokerTx = make(map[string][]string)
	b.ChainConfig = pcc
	b.BrokerAddress = b.initBrokerAddr(params.BrokerNum)
	b.BrokerBalance = make(map[string]ShardBalanceInt)
	b.LockBalance = make(map[string]ShardBalanceInt)
	b.ProfitBalance = make(map[string]ShardBalanceFloat)
	for _, val := range params.Transaction_Types {
		b.BrokerBalance[val] = b.initBrokerBalance(params.Init_broker_Balance)
		b.LockBalance[val] = b.initBrokerBalance(big.NewInt(0))
		b.ProfitBalance[val] = b.initFloatBalance(big.NewFloat(0))
	}
	b.Brokerage = big.NewFloat(params.Brokerage)
}

func (b *Broker) IsBroker(address string) bool {
	for _, brokerAddress := range b.BrokerAddress {
		if brokerAddress == address {
			return true
		}
	}
	return false
}

func (b *Broker) initBrokerAddr(num int) []string {
	brokerAddress := make([]string, 0)
	filePath := `./broker/broker`
	readFile, err := os.Open(filePath)
	if err != nil {
		fmt.Println(err)
	}
	fileScanner := bufio.NewScanner(readFile)
	fileScanner.Split(bufio.ScanLines)
	for fileScanner.Scan() {
		address := fileScanner.Text()
		brokerAddress = append(brokerAddress, address)
		num--
		if num == 0 {
			break
		}
	}

	readFile.Close()
	return brokerAddress
}

func (b *Broker) initBrokerBalance(balance *big.Int) map[string]map[uint64]*big.Int {
	BrokerBalance := make(map[string]map[uint64]*big.Int)
	for _, address := range b.BrokerAddress {
		BrokerBalance[address] = make(map[uint64]*big.Int)
		for sid := uint64(0); sid < uint64(params.ShardNum); sid++ {
			BrokerBalance[address][sid] = new(big.Int).Set(balance)
		}
	}
	return BrokerBalance
}

func (b *Broker) initFloatBalance(balance *big.Float) map[string]map[uint64]*big.Float {
	BrokerBalance := make(map[string]map[uint64]*big.Float)
	for _, address := range b.BrokerAddress {
		BrokerBalance[address] = make(map[uint64]*big.Float)
		for sid := uint64(0); sid < uint64(params.ShardNum); sid++ {
			BrokerBalance[address][sid] = new(big.Float).Set(balance)
		}
	}
	return BrokerBalance
}
