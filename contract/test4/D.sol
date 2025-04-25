import "./E.sol";

contract D {

    uint256 public a;
    address public addr;
    constructor() {
        a=10;
        //创建合约D时，递归创建合约E
        E e = new E();
        addr = address(e);
    }

    function getA() public returns(uint256) {
        //递归调用合约E的getA()函数
        return E(addr).getA()+a;
    }
}