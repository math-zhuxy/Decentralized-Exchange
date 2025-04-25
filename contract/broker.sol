pragma solidity ^0.8.0;

contract BrokerRegistry {
    // 定义一个结构体来存储broker的信息
    struct Broker {
        address brokerAddress;
        uint256 collateralAmount;
        bool isActive;
    }

    // 定义一个映射来存储所有的broker
    mapping(address => Broker) public brokers;

    // 定义受信任的第三方地址（这里可以是一个多签钱包或智能合约地址）
    address public trustedThirdParty;

    // 事件，用于记录broker的注册和注销
//    event BrokerRegistered(address indexed brokerAddress, uint256 collateralAmount);
//    event BrokerDeregistered(address indexed brokerAddress);

    // 构造函数，初始化受信任的第三方地址
    constructor() {
        trustedThirdParty = msg.sender;
    }

    // 用户请求成为broker的函数
    function requestBrokerStatus() external payable {
        //require(msg.value == _collateralAmount, "Collateral amount must match the sent amount");
        require(brokers[msg.sender].isActive == false, "Address is already a broker");

        // 将抵押资产发送到受信任的第三方地址
        //(bool success, ) = trustedThirdParty.call{value: _collateralAmount}("");
        //require(success, "Failed to transfer collateral to trusted third party");

        // 更新broker状态
        brokers[msg.sender] = Broker({
            brokerAddress: msg.sender,
            collateralAmount: 0,
            isActive: true
        });

        // 触发事件
        //emit BrokerRegistered(msg.sender, _collateralAmount);
    }

    // broker注销并取回抵押资产的函数（这里假设需要受信任第三方的批准）
    function deregisterBroker(address _brokerAddress, uint256 _collateralAmount, bool _approvedByThirdParty) external {
        require(msg.sender == trustedThirdParty, "Only the trusted third party can call this function");
        Broker storage broker = brokers[_brokerAddress];

        require(broker.isActive, "Address is not a broker");
        require(broker.collateralAmount == _collateralAmount, "Collateral amount does not match");
        require(_approvedByThirdParty, "Approval by trusted third party is required");

        // 更新broker状态
        broker.isActive = false;

        // 这里假设抵押资产已经从受信任第三方退回（实际实现中需要处理这部分逻辑）
        // (Note: This is a simplified version. In a real-world scenario, you would need to handle the collateral refund properly.)

        // 触发事件
        //emit BrokerDeregistered(_brokerAddress);
    }

    // 获取broker信息的函数
    function getBrokerInfo(address _brokerAddress) external view returns (address, uint256, bool) {
        Broker storage broker = brokers[_brokerAddress];
        return (broker.brokerAddress, broker.collateralAmount, broker.isActive);
    }
}