contract C1 {

    uint256 public a;

    constructor() {
        a=10;
    }

    function getA(uint b) public returns(uint256) {
        return a;
    }

}
