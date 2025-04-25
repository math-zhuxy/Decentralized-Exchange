import "./B.sol";
contract A {

    uint256 public a;
    address public addr;

    constructor() {

    }

    function create() public returns(uint256){
        B b = new B();
        addr = address(b);
        a = B(addr).getA(123) +10;
        return a;
    }

    function getA() public returns(uint256){
        return a;
    }
}

