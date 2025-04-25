contract B2 {

    uint256 public a;
    constructor() {
        a=10;
    }

    function getA() public returns(uint256) {
        return a;
    }

    function testError() public returns(uint256){
        a=20;
        uint256 x = 1;
        uint256 y = 0;
        assert(x / y == 1); // 除零错误，交易回滚
        a=30;
        return a;
    }

}
