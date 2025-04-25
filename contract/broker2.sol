// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.0;



contract Broker {
    // define broker struct
    struct Broker {
        address brokerAddress;
        uint256 stakeAmount;
        bool isActive;
    }

    // a map to store all brokers
    mapping(address => Broker) public brokers;

    // B2E protocol address
    address public B2EAddr;

    // event to record the register and deregister of broker
    event BrokerRegistered(address indexed brokerAddress);
    event BrokerDeregistered(address indexed brokerAddress);

    // B2E manager initialize the B2E address
    constructor() {
        B2EAddr = msg.sender;
    }

    // user apply to become broker
    function requestBroker() external payable {
        //require not a breoker before
        require(brokers[msg.sender].isActive == false, "Address is already a broker");

        // update broker status
        brokers[msg.sender] = Broker({
            brokerAddress: msg.sender,
            collateralAmount: 0,
            isActive: true
        });

        // emit the register event
        emit BrokerRegistered(msg.sender);
    }

    // withdraw broker
    function deregisterBroker(address _brokerAddress, uint256 _collateralAmount) external {
        //only the B2E manager can invoke this function
        require(msg.sender == B2EAddr, "Only the B2E manager can call this function");
        Broker storage broker = brokers[_brokerAddress];

        require(broker.isActive, "Address is not a broker");
        require(broker.stakeAmount == _collateralAmount, "Collateral amount does not match");


        // update broker status
        broker.isActive = false;

        // transfer the principal and profit from B2E to user
        _brokerAddress.call{value: _collateralAmount}("");

        // emit the broker deregister event
        emit BrokerDeregistered(_brokerAddress);
    }

    // get broker information
    function getBrokerInfo(address _brokerAddress) external view returns (address, uint256, bool) {
        Broker storage broker = brokers[_brokerAddress];
        return (broker.brokerAddress, broker.stakeAmount, broker.isActive);
    }
}