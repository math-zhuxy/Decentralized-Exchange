contract E {

    uint256 public a;

    constructor() {
        a=10;
    }

    function getA() public returns(uint256) {
        return a;
    }
}