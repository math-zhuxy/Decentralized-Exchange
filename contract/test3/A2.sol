import "./B2.sol";
contract A2 {

    uint256 public a;
    address public addr;

    constructor() payable {
        test();
    }

    function test() public {
        B2 b = new B2();
        addr = address(b);
    }

    function getA() public returns(uint256){
        return a + B2(addr).getA() ;
    }

    function testError() public {
        a = 1;
        a= B2(addr).testError() + 2;

    }
}
