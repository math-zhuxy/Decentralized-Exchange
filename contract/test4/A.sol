import "./B.sol";
contract A {

    uint256 public a;
    address public addr;

    constructor() payable {
        a=10;
        //创建合约A时，递归创建合约B
        B b = new B();
        addr = address(b);
    }

    function getA() public returns(uint256){
        //递归调用合约B的getA()函数
        return a + B(addr).getA() ;
    }
}