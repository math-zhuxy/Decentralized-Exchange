pragma solidity ^0.8.0;

contract ContractA {
    // 事件用于记录余额变化
    event BalanceChanged(address indexed account, uint256 newBalance);

    // 接受转账的 fallback 函数（在 Solidity 0.8.0 及更高版本中，fallback 函数被重命名为 receive()）
    receive() external payable {
        _changeBalance(msg.sender);
    }


    // 由于合约账户不能直接“拥有”其他账户的余额，这个函数实际上返回合约本身的余额
    function getContractBalance() public view returns (uint256) {
        return address(this).balance;
    }

    // 向另一个合约或地址转账的函数
    function withdraw(address payable to, uint256 amount) public payable {
//        require(amount <= address(this).balance, "Insufficient balance");
        to.transfer(amount);
        _changeBalance(address(this));
    }


    function test(address payable to) public payable {
         withdraw(address,1);
        withdraw(address,999);
         withdraw(address,3);
    }

    // 内部函数，用于记录余额变化事件
    function _changeBalance(address account) internal {
        emit BalanceChanged(account, address(this).balance);
    }
}