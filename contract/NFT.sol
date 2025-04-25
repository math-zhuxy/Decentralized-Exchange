contract NFT{
    // NFT基础信息（所有持有者共享）
    struct NFTMetadata {
        uint256 nftId;
        string name;
        string imageUrl;
        uint256 totalShares; //NFT的总份数，不会被更改
    }

    // 用户基础持有信息
    struct UserHolding {
        uint256 totalShares; //用户拥有的份数
        uint256 availableShares; //未上架的份数
    }


    // 上架订单结构
    struct Listing {
        uint256 listingId;
        uint256 nftId;
        address seller;
        uint256 shares;
        uint256 price;
    }


    // 核心数据结构
    uint256 private nftIdCounter;
    uint256 private listingIdCounter;

    mapping(uint256 => NFTMetadata) public nftMetadata; // NFT基础信息
    mapping(uint256 => Listing) public listings; // 所有上架订单
    mapping(uint256 => mapping(address => UserHolding)) public userHolding; // 所有用户持有详情

    // 索引映射 便于用户查询
    mapping(address => uint256[]) private userOwnedNFTs; // 用户地址 => 持有的NFT ID数组
    mapping(address => uint256[]) private userListings; // 用户的上架订单


    //铸造NFT 返回铸造的NFT的Id
    function mint(address payable userId, string calldata name, string calldata url, uint256 totalShares) external returns(uint256) {

        uint256 gas = 10;//假设gas为10
        //检查用户是否有足够多的钱
        require(userId.balance >= gas, "Insufficient balance for minting");


        nftIdCounter++;
        nftMetadata[nftIdCounter] = NFTMetadata(nftIdCounter,name,url,totalShares);
        userHolding[nftIdCounter][userId] = UserHolding(totalShares, totalShares);
        userOwnedNFTs[userId].push(nftIdCounter);
        return nftIdCounter;
    }



    //返回用户拥有的NFT的所有信息（包括上架的和未上架的）
    function getMyNFTs() external view returns (
        uint256[] memory nftIds,
        string[] memory names,
        string[] memory imageUrls,
        uint256[] memory sharesList,
        uint256[] memory pricesList,
        bool[] memory listingStatus,
        uint256[] memory listingIds
    ) {
        address user = msg.sender;

        // 第一阶段：预加载元数据到结构体数组
        NFTMetadata[] memory metadataCache = new NFTMetadata[](userOwnedNFTs[user].length);
        uint256[] storage ownedNFTIds = userOwnedNFTs[user];

        for(uint i = 0; i < ownedNFTIds.length; i++) {
            uint256 nftId = ownedNFTIds[i];
            metadataCache[i] = NFTMetadata({
                nftId: nftId,
                name: nftMetadata[nftId].name,
                imageUrl: nftMetadata[nftId].imageUrl,
                totalShares: nftMetadata[nftId].totalShares
            });
        }

        // 第二阶段：计算总条目数
        uint256 totalItems = 0;

        // 1. 统计有效挂单数
        for(uint i = 0; i < userListings[user].length; i++) {
            if(listings[userListings[user][i]].shares > 0) {
                totalItems++;
            }
        }

        // 2. 统计未上架数
        for(uint i = 0; i < ownedNFTIds.length; ) {
            uint256 nftId = ownedNFTIds[i];

            if(userHolding[nftId][user].availableShares > 0) {
                totalItems++;
            }

            unchecked { i++; }
        }


        // 第三阶段：填充数据
        nftIds = new uint256[](totalItems);
        names = new string[](totalItems);
        imageUrls = new string[](totalItems);
        sharesList = new uint256[](totalItems);
        pricesList = new uint256[](totalItems);
        listingStatus = new bool[](totalItems);
        listingIds = new uint256[](totalItems);

        uint256 index = 0;

        // A. 处理挂单
        for(uint i = 0; i < userListings[user].length; i++) {
            Listing storage l = listings[userListings[user][i]];
            if(l.shares == 0) continue;

            // 从缓存数组查找元数据
            NFTMetadata memory meta;
            for(uint j = 0; j < metadataCache.length; j++) {
                if(metadataCache[j].nftId == l.nftId) {
                    meta = metadataCache[j];
                    break;
                }
            }

            nftIds[index] = l.nftId;
            names[index] = meta.name;
            imageUrls[index] = meta.imageUrl;
            sharesList[index] = l.shares;
            pricesList[index] = l.price;
            listingStatus[index] = true;
            listingIds[index] = l.listingId;
            index++;
        }

        // B. 处理未上架
        for(uint i = 0; i < ownedNFTIds.length; ) {
            uint256 nftId = ownedNFTIds[i];

            if(userHolding[nftId][user].availableShares > 0) {
                NFTMetadata memory meta = metadataCache[i];

                nftIds[index] = nftId;
                names[index] = meta.name;
                imageUrls[index] = meta.imageUrl;
                sharesList[index] = userHolding[nftId][user].availableShares;
                pricesList[index] = 0;
                listingStatus[index] = false;
                listingIds[index] = 0; //标识未上架
                index++;
            }

            unchecked { i++; }
        }
    }



    function listNFT(uint256 nftId,uint256 shares,uint256 price) external {
        require(userHolding[nftId][msg.sender].availableShares >= shares, "Insufficient available shares");

        // 检查是否已有相同价格的挂单，有则合并
        bool merged = false;
        for(uint i=0; i<userListings[msg.sender].length; i++){
            Listing storage existing = listings[userListings[msg.sender][i]];
            if(existing.nftId == nftId && existing.price == price && existing.seller == msg.sender) {
                existing.shares += shares;
                merged = true;
                break;
            }
        }

        // 创建新订单
        if(!merged) {
            listingIdCounter++;
            listings[listingIdCounter] = Listing({
                listingId: listingIdCounter,
                nftId: nftId,
                seller: msg.sender,
                shares: shares,
                price: price
            });
            userListings[msg.sender].push(listingIdCounter);
        }

        // 更新持有量
        userHolding[nftId][msg.sender].availableShares -= shares;
    }



    function unlistNFT(uint256 listingId) external {
        Listing storage listing = listings[listingId];
        require(listing.seller == msg.sender, "Not owner");

        // 恢复持有量
        userHolding[listing.nftId][msg.sender].availableShares += listing.shares;

        // 从用户挂单列表移除
        removeFromArray(userListings[msg.sender], listingId);
        delete listings[listingId];
    }

    function removeFromArray(uint256[] storage arr, uint256 id) internal {
        for(uint i = 0; i < arr.length; i++) {
            if(arr[i] == id) {
                // 当元素不是最后一个时，才需要交换
                if(i != arr.length - 1) {
                    arr[i] = arr[arr.length - 1];
                }
                arr.pop();
                return; // 找到后立即返回
            }
        }
    }



    function getAllMarketListings() external view returns (
        address[] memory sellers,
        uint256[] memory nftIds,
        string[] memory names,
        string[] memory imageUrls,
        uint256[] memory sharesList,
        uint256[] memory pricesList,
        uint256[] memory listingIds
    ) {
        // 先计算有效挂单数量
        uint validCount = 0;
        for (uint listingId = 1; listingId <= listingIdCounter; listingId++) {
            if (isValidListing(listingId)) {
                validCount++;
            }
        }

        // 初始化返回数组
        sellers = new address[](validCount);
        nftIds = new uint256[](validCount);
        names = new string[](validCount);
        imageUrls = new string[](validCount);
        sharesList = new uint256[](validCount);
        pricesList = new uint256[](validCount);
        listingIds = new uint256[](validCount);

        // 阶段2：填充有效数据
        uint index = 0;
        for (uint listingId = 1; listingId <= listingIdCounter; listingId++) {
            if (isValidListing(listingId)) {
                Listing storage l = listings[listingId];
                NFTMetadata storage meta = nftMetadata[l.nftId];

                sellers[index] = l.seller;
                nftIds[index] = l.nftId;
                names[index] = meta.name;
                imageUrls[index] = meta.imageUrl;
                sharesList[index] = l.shares;
                pricesList[index] = l.price;
                listingIds[index] = listingId;
                index++;
            }
        }
    }

    // 验证函数
    function isValidListing(uint256 listingId) private view returns (bool) {
        Listing storage l = listings[listingId];
        return
            l.seller != address(0) &&
            l.shares > 0 &&
            l.seller != msg.sender;
    }

    function buyNFT(uint256 listingId, uint256 shares) external payable {
        Listing storage listingNFT = listings[listingId];
        require(listingNFT.seller != msg.sender, "Cannot purchase your own listing");
        require(listingNFT.shares >= shares, "Insufficient shares");

        uint256 totalPrice = listingNFT.price * shares;
        require(msg.value >= totalPrice, "Insufficient funds");//这里还要考虑gasfee

        //msg.sender.balance（我不确定测试用的remix vm是不是用value，然后brokerchain的是用balance）


        // 安全转账
        (bool sent, ) = payable(listingNFT.seller).call{value: totalPrice}("");
        require(sent, "Transfer failed");

        // 处理超额ETH
        if(msg.value > totalPrice) {
            payable(msg.sender).transfer(msg.value - totalPrice);
        }


        // 维护买卖双方持有列表
        updateOwnership(msg.sender, listingNFT.nftId, int256(shares));
        updateOwnership(listingNFT.seller, listingNFT.nftId, -int256(shares));

        // 更新订单状态
        listingNFT.shares -= shares;

        // 处理完全售罄的订单，更新挂单列表
        if(listingNFT.shares == 0) {
            removeFromArray(userListings[listingNFT.seller], listingId);
            delete listings[listingId];
        }
    }


    function updateOwnership(address user, uint256 nftId, int256 shares) internal {
        UserHolding storage holding = userHolding[nftId][user];

        if (shares > 0) {
            // 买入逻辑
            holding.totalShares += uint256(shares);
            holding.availableShares += uint256(shares);

            if (holding.totalShares == uint256(shares)) {
                userOwnedNFTs[user].push(nftId);
            }
        } else {
            // 卖出逻辑
            uint256 decreaseAmount = uint256(-shares);

            require(holding.availableShares >= decreaseAmount || holding.totalShares - holding.availableShares >= decreaseAmount,"Overflow");

            holding.totalShares -= decreaseAmount;

            // 完全清空时执行清理
            if (holding.totalShares == 0) {
                removeFromOwnedList(user, nftId);
                delete userHolding[nftId][user];
            }
        }
    }

    function removeFromOwnedList(address user, uint256 nftId) internal {
        uint256[] storage list = userOwnedNFTs[user];
        for(uint i = 0; i < list.length; i++) {
            if(list[i] == nftId) {
                if(i != list.length - 1) {
                    list[i] = list[list.length - 1];
                }
                list.pop();
                return;
            }
        }
    }

}