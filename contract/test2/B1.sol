import "./C1.sol";
contract B1 {

    uint256 public a;
    address public addr;
    constructor() {
        a=10;
        C1 c = new C1();
        addr = address(c);
        a = a + C1(addr).getA(123456) +10;
    }

    function getA(uint b) public returns(uint256) {
        return a;
    }

}
