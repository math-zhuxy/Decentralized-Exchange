package test

import "testing"

func TestKey(t *testing.T) {

	//// 生成 RSA 密钥对
	//privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	//if err != nil {
	//	log.Fatalf("生成 RSA 密钥对失败: %v", err)
	//}
	//// 将私钥转换为 ASN.1 PKCS#1 DER 编码
	//privDER := x509.MarshalPKCS1PrivateKey(privateKey)
	//// 将 DER 编码的私钥转换为 PEM 格式
	//privPEM := pem.EncodeToMemory(&pem.Block{
	//	Type:  "RSA PRIVATE KEY",
	//	Bytes: privDER,
	//})
	//// 将公钥提取为 *rsa.PublicKey 类型
	//publicKey := &privateKey.PublicKey
	//// 将公钥转换为 ASN.1 PKIX DER 编码
	//pubDER, err := x509.MarshalPKIXPublicKey(publicKey)
	//if err != nil {
	//	log.Fatalf("公钥编码失败: %v", err)
	//}
	//// 将 DER 编码的公钥转换为 PEM 格式
	//pubPEM := pem.EncodeToMemory(&pem.Block{
	//	Type:  "RSA PUBLIC KEY",
	//	Bytes: pubDER,
	//})
	//// 打印私钥和公钥
	//fmt.Println("私钥:")
	//fmt.Println(string(privPEM))
	//fmt.Println("公钥:")
	//fmt.Println(string(pubPEM))

	//message := []byte("路多辛的博客")
	//
	//// 使用公钥和 OAEP 填充方案加密数据
	//label := []byte("OAEP Encrypted")
	//ciphertext, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, publicKey, message, label)
	//if err != nil {
	//	fmt.Fprintf(os.Stderr, "Error encrypting message: %s\n", err)
	//	return
	//}
	//fmt.Printf("Ciphertext: %x\n", ciphertext)
	//
	//// 使用私钥和 OAEP 填充方案解密数据
	//plaintext, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, privateKey, ciphertext, label)
	//if err != nil {
	//	fmt.Fprintf(os.Stderr, "Error decrypting message: %s\n", err)
	//	return
	//}
	//fmt.Printf("Plaintext: %s\n", plaintext)

	//lines1 := strings.Split(string(privPEM), "\n")
	//lines2 := strings.Split(string(pubPEM), "\n")
	//
	//// 去掉第一行和最后一行
	//if len(lines1) > 2 {
	//	lines1 = lines1[1 : len(lines1)-2]
	//}
	//if len(lines2) > 2 {
	//	lines2 = lines2[1 : len(lines2)-2]
	//}
	//// 将剩余的行合并为一个字符串，不移除空白字符（如果有的话）
	//// 但在这个场景中，由于我们只关心去掉换行符，所以直接join即可
	//oneLineKey1 := strings.Join(lines1, "")
	//oneLineKey2 := strings.Join(lines2, "")
	//
	//// 输出结果
	//fmt.Println(oneLineKey1)
	//fmt.Println(oneLineKey2)
	//
	//privateBase64 := oneLineKey1
	//pubBase64 := oneLineKey2
	//b1, err := base64.StdEncoding.DecodeString(privateBase64)
	//b2, err := base64.StdEncoding.DecodeString(pubBase64)
	//if err != nil {
	//	log.Fatal(err)
	//}
	//privateHex := hex.EncodeToString(b1)
	//pubHex := hex.EncodeToString(b2)
	//fmt.Println(privateHex)
	//fmt.Println(pubHex)
	//
	//pemPrivateKey := privateHex
	//pemPublicKey := pubHex

	// 解析 PEM 编码的私钥
	//block, _ := pem.Decode([]byte(privPEM))
	//if block == nil || block.Type != "RSA PRIVATE KEY" {
	//	fmt.Println("failed to decode PEM block containing private key")
	//	return
	//}
	//
	//privateKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	//if err != nil {
	//	fmt.Fprintf(os.Stderr, "Error parsing private key: %s\n", err)
	//	return
	//}
	//
	//fmt.Println("Private Key:", privateKey)
	//
	//// 解析 PEM 编码的公钥
	//block, _ = pem.Decode([]byte(pubPEM))
	//if block == nil || block.Type != "RSA PUBLIC KEY" {
	//	fmt.Println("failed to decode PEM block containing public key")
	//	return
	//}
	//
	//pubKeyInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	//if err != nil {
	//	fmt.Fprintf(os.Stderr, "Error parsing public key: %s\n", err)
	//	return
	//}
	//
	//publicKey, ok := pubKeyInterface.(*rsa.PublicKey)
	//if !ok {
	//	fmt.Fprintf(os.Stderr, "Error casting public key to RSA Public Key\n")
	//	return
	//}
	//
	//fmt.Println("Public Key:", publicKey)

	//privDER1 := x509.MarshalPKCS1PrivateKey(privateKey)
	//// 将 DER 编码的私钥转换为 PEM 格式
	//privPEM1 := pem.EncodeToMemory(&pem.Block{
	//	Type:  "RSA PRIVATE KEY",
	//	Bytes: privDER1,
	//})
	//// 将公钥提取为 *rsa.PublicKey 类型
	//publicKey1 := &privateKey.PublicKey
	//// 将公钥转换为 ASN.1 PKIX DER 编码
	//pubDER1, err := x509.MarshalPKIXPublicKey(publicKey1)
	//if err != nil {
	//	log.Fatalf("公钥编码失败: %v", err)
	//}
	//// 将 DER 编码的公钥转换为 PEM 格式
	//pubPEM1 := pem.EncodeToMemory(&pem.Block{
	//	Type:  "RSA PUBLIC KEY",
	//	Bytes: pubDER1,
	//})
	//// 打印私钥和公钥
	//fmt.Println("私钥:")
	//fmt.Println(string(privPEM1))
	//fmt.Println("公钥:")
	//fmt.Println(string(pubPEM1))

	//return

	//fmt.Println(uint64(utils.Addr2Shard("b1aa5a0ac6761fe4fcb6aca7dfa7e7c800000002"))) //2
	//fmt.Println(uint64(utils.Addr2Shard("b1aa5a0ac6761fe4fcb6aca7dfa7e7c800000003"))) //0
}
