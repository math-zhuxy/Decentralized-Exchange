import "./B1.sol";
contract A1 {

    uint256 public a;
    address public addr;

    constructor() {

    }

    function test() public returns(uint256){
        B1 b = new B1();
        addr = address(b);
        a = B1(addr).getA(123) +10;
        return a;
    }

    function getA() public returns(uint256){
        return a;
    }
}

