import "./D.sol";
contract C {

    uint256 public a;
    address public addr;
    constructor() {
        a=10;
        //创建合约C时，递归创建合约D
        D d = new D();
        addr = address(d);
    }

    function getA() public returns(uint256) {
        //递归调用合约D的getA()函数
        return D(addr).getA()+a;
    }
}