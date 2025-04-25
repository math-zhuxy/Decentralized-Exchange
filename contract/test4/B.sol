import "./C.sol";
contract B {

    uint256 public a;
    address public addr;
    constructor() {
        a=10;
        //创建合约B时，递归创建合约C
        C c = new C();
        addr = address(c);
    }

    function getA() public returns(uint256) {
        //递归调用合约C的getA()函数
        return a+C(addr).getA();
    }
}