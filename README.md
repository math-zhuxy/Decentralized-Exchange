# Preface
This code is the implementation for the project of BVM + Brokerchain.</br>
The code supports running on a single machine or multiple machines, and you can modify the relevant settings through the configuration file.

# Installation
1. Get the latest code from [https://github.com/sysu-xbz/Brokerchain-contract](https://github.com/sysu-xbz/Brokerchain-contract) (The recent version of the repository is already included in the current directory. Due to intellectual property protection, we have not made this repository public for the time being. If we are happy, we will make the repository public.)
2. For Linux: `cd Brokerchain-contract & go build -o blockEmulator_Linux_Precompile ./main.go`</br>
    For Windows: `cd Brokerchain-contract & go build -o blockEmulator_Windows_Precompile.exe ./main.go`
   The first compilation will automatically pull the required dependencies, which might take some time. (If the download is too slow, it might be necessary to change the source, try setting the goproxy: `export GOPROXY=https://goproxy.cn` and then pull the dependencies.)

# Configuration
+ **Parameter File Settings**: `"brokerchain-contract/params/global_config.go"`
```text
var (
	Block_Interval      = 1000   // generate new block interval
	MaxBlockSize_global = 500000 // the block contains the maximum number of transactions
	InjectSpeed         = 5000   // the transaction inject speed
	TotalDataSize       = 500000 // the total number of txs
	BatchSize           = 5000   // supervisor read a batch of txs then send them, it should be larger than inject speed
	BrokerNum           = 1
	NodesInShard        = 4
	ShardNum            = 4
	IterNum_B2E         = 5
	Brokerage           = 0.1
	DataWrite_path      = "./result/" // measurement data result output path
	LogWrite_path       = "./log"     // log output path
	KeyWrite_path       = "./key"
	SupervisorAddr      = "127.0.0.1:18800"                                                           //supervisor ip address
    NodeID              uint64
	ShardID             uint64
)
```

# Execution
When actually running, start the script `brokerchain-contract/bat_shardNum=3_NodeNum=4_mod=Broker_b2e.sh`(for linux) or `brokerchain-contract/bat_shardNum=3_NodeNum=4_mod=Broker_b2e.bat`(for Windows) . This script will automatically start 3 shards, modify the configuration file in each node, and run the 4 nodes in each shard.
Once the machine is running the script, BVM + Brokerchian will start operating. You can check the node logs in the `brokerchain-contract/log` folder. 

# Module Overview

## committee Module
Maintains the transaction pool, gets the status from the shard, packages blocks, executes transactions.

## core Module
Defines various basic types, such as transactions, blocks, blockchains, etc.

## pbft Module
Implementation of the pbft protocol.

## utils Module
Commonly used utility functions.

## vm Module
Implementation of Broker VM (BVM)

# Issues
For any issues, contact the author at xiebzh3@mail2.sysu.edu.cn
