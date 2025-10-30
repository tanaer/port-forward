package xray

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/curve25519"
)

// RealityKeyPair Reality密钥对
type RealityKeyPair struct {
	PrivateKey string
	PublicKey  string
}

// GenerateRealityKeys 生成Reality密钥对
func GenerateRealityKeys() (*RealityKeyPair, error) {
	// 生成32字节私钥
	privateKey := make([]byte, curve25519.ScalarSize)
	if _, err := rand.Read(privateKey); err != nil {
		return nil, fmt.Errorf("生成私钥失败: %v", err)
	}

	// 计算公钥
	publicKey, err := curve25519.X25519(privateKey, curve25519.Basepoint)
	if err != nil {
		return nil, fmt.Errorf("计算公钥失败: %v", err)
	}

	return &RealityKeyPair{
		PrivateKey: base64.RawURLEncoding.EncodeToString(privateKey),
		PublicKey:  base64.RawURLEncoding.EncodeToString(publicKey),
	}, nil
}

// GenerateShortId 生成Reality ShortId
func GenerateShortId() (string, error) {
	// 生成8字节随机ID
	shortId := make([]byte, 8)
	if _, err := rand.Read(shortId); err != nil {
		return "", fmt.Errorf("生成ShortId失败: %v", err)
	}

	return fmt.Sprintf("%x", shortId), nil
}

// RealityDomains 推荐的Reality回落域名列表
var RealityDomains = []struct {
	Domain      string
	Description string
}{
	{"microsoft.com:443", "微软"},
	{"yahoo.com:443", "雅虎"},
	{"apple.com:443", "苹果"},
	{"cloudflare.com:443", "Cloudflare"},
	{"aws.amazon.com:443", "亚马逊AWS"},
	{"www.tesla.com:443", "特斯拉"},
	{"www.cisco.com:443", "思科"},
	{"www.oracle.com:443", "甲骨文"},
	{"www.ibm.com:443", "IBM"},
	{"www.samsung.com:443", "三星"},
}

// GetRealityDomainList 获取Reality域名列表
func GetRealityDomainList() []string {
	domains := make([]string, len(RealityDomains))
	for i, d := range RealityDomains {
		domains[i] = d.Domain
	}
	return domains
}
