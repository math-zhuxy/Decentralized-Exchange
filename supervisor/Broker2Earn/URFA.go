package Broker2Earn

import (
	"blockEmulator/core"
	"blockEmulator/message"
	"blockEmulator/utils"
	"math/big"
	"sort"
)

// Relax functin
// Input tx list and brokerTable (brokers balance)
// Output a map that ctx -> broker

func URFA_Linear(brokerRawMegs []*message.BrokerRawMeg, BrokerBalance map[string]*big.Int, alpha int, sigma float64) []*RatioBrokerRawMeg {

	// get all broker address
	brokerAddresses := make([]string, 0, len(BrokerBalance))

	for brokerAddress := range BrokerBalance {
		brokerAddresses = append(brokerAddresses, brokerAddress)
	}
	// build tmp broker balance
	nowBrokerBalance := make(map[string]*big.Int)
	for brokerID, _ := range BrokerBalance {
		nowBrokerBalance[brokerID] = new(big.Int).SetInt64(0)
	}

	// deal txs
	txs := make([]*core.Transaction, 0)
	for _, brokerRawMeg := range brokerRawMegs {
		txs = append(txs, brokerRawMeg.Tx)
	}

	// sort tx by Fee/Value
	sort.Slice(txs, func(i, j int) bool {
		// tranform to big.float
		div1 := new(big.Float).SetInt(txs[i].Value)
		div1.Add(div1, big.NewFloat(0.000001))
		div1.Quo(new(big.Float).SetInt(txs[i].Fee), div1)

		div2 := new(big.Float).SetInt(txs[j].Value)
		div2.Add(div2, big.NewFloat(0.000001))
		div2.Quo(new(big.Float).SetInt(txs[j].Fee), div2)

		return div1.Cmp(div2) > 0
	})

	// allocate tx to broker 1 by 1

	result := make([]*RatioBrokerRawMeg, 0)
	brokerIndex := 0
	for _, tx := range txs {
		if brokerIndex >= len(brokerAddresses) {
			break
		}
		brokerB := nowBrokerBalance[brokerAddresses[brokerIndex]]
		brokerC := BrokerBalance[brokerAddresses[brokerIndex]]
		// if tx value bigger than broker, jump
		if brokerC.Cmp(tx.Value) < 0 {
			continue
		}

		tmp_brokerRawMeg := &RatioBrokerRawMeg{
			Tx:          tx,
			BrokerRatio: make(map[utils.Address]float64),
		}

		tmpValue := new(big.Int).SetInt64(0)
		// if tx value + allocated broker > broker balance, split tx
		if brokerB.Cmp(brokerC) < 0 && tmpValue.Add(brokerB, tx.Value).Cmp(brokerC) >= 0 {

			//println("tx address A ", tx.TxHash, " ", tx.Value.String(), " ", brokerB.String(), " ", brokerC.String())
			// full the broker
			brokerB.Set(brokerC)

			// search the rest part of tx belong to
			sub := new(big.Int).SetInt64(0)
			sub.Sub(tmpValue, brokerC)

			// calulate ratio
			div1 := new(big.Float).SetInt(sub)
			div2 := new(big.Float).SetInt(tx.Value)

			ratio, _ := div1.Quo(div1, div2).Float64()

			//println("2brokerIndex is ", brokerIndex, " / ", len(brokerAddresses))
			tmp_brokerRawMeg.BrokerRatio[brokerAddresses[brokerIndex]] = 1.0 - ratio

			brokerIndex += 1
			if brokerIndex >= len(brokerAddresses) {
				break
			}
			for j := brokerIndex; j < len(brokerAddresses); j++ {
				brokerB1 := nowBrokerBalance[brokerAddresses[brokerIndex]]
				brokerC1 := BrokerBalance[brokerAddresses[brokerIndex]]
				tmpValue1 := new(big.Int)
				tmpValue1.Add(brokerB1, sub)

				if brokerC1.Cmp(tmpValue1) < 0 {
					continue
				}
				tmp_brokerRawMeg.BrokerRatio[brokerAddresses[j]] = ratio
				break

			}

		} else {
			//println("tx address B ", tx.TxHash, " ", tx.Value.String(), " ", brokerB.String(), " ", brokerC.String())
			nowBrokerBalance[brokerAddresses[brokerIndex]].Add(brokerB, tx.Value)
			tmp_brokerRawMeg.BrokerRatio[brokerAddresses[brokerIndex]] = 1.0
		}
		result = append(result, tmp_brokerRawMeg)

	}
	return result
}
