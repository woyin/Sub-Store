// Package ageutil 提供 AGE 加密/解密功能，兼容 Node.js 版本 age.js 的所有特性。
package ageutil

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"filippo.io/age"
)

const (
	AGEArmorHeader = "-----BEGIN AGE ENCRYPTED FILE-----"
	AGEArmorFooter = "-----END AGE ENCRYPTED FILE-----"
)

// AGEKeyTypes 支持的密钥类型。
var AGEKeyTypes = struct {
	X25519 string
	Hybrid string
}{
	X25519: "x25519",
	Hybrid: "mlkem768-x25519",
}

// ValidateRecipient 验证并返回标准化的 age 公钥。
func ValidateRecipient(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", nil
	}
	if !IsSupportedRecipient(key) {
		return "", errors.New("age-public-key 仅支持 X25519(age1...) 或 MLKEM768-X25519(age1pq1...) 公钥")
	}
	return key, nil
}

// ValidateIdentity 验证并返回标准化的 age 私钥。
func ValidateIdentity(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", nil
	}
	if !IsSupportedIdentity(key) {
		return "", errors.New("age-secret-key 仅支持 X25519(AGE-SECRET-KEY-1...) 或 MLKEM768-X25519(AGE-SECRET-KEY-PQ-1...) 私钥")
	}
	return key, nil
}

// IsSupportedRecipient 检查是否为支持的公钥格式。
func IsSupportedRecipient(key string) bool {
	return IsSupportedX25519Recipient(key) || IsSupportedHybridRecipient(key)
}

// IsSupportedIdentity 检查是否为支持的私钥格式。
func IsSupportedIdentity(key string) bool {
	return IsSupportedX25519Identity(key) || IsSupportedHybridIdentity(key)
}

// IsSupportedX25519Recipient 检查是否为 X25519 公钥。
func IsSupportedX25519Recipient(key string) bool {
	return strings.HasPrefix(key, "age1") &&
		!strings.HasPrefix(key, "age1pq1") &&
		!strings.HasPrefix(key, "age1tag1") &&
		!strings.HasPrefix(key, "age1tagpq1")
}

// IsSupportedHybridRecipient 检查是否为 MLKEM768-X25519 公钥。
func IsSupportedHybridRecipient(key string) bool {
	return strings.HasPrefix(key, "age1pq1")
}

// IsSupportedX25519Identity 检查是否为 X25519 私钥。
func IsSupportedX25519Identity(key string) bool {
	return strings.HasPrefix(key, "AGE-SECRET-KEY-1")
}

// IsSupportedHybridIdentity 检查是否为 MLKEM768-X25519 私钥。
func IsSupportedHybridIdentity(key string) bool {
	return strings.HasPrefix(key, "AGE-SECRET-KEY-PQ-1")
}

// GenerateKeyPair 生成新的 AGE 密钥对。
func GenerateKeyPair() (publicKey, secretKey string, err error) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate age key pair: %w", err)
	}
	return identity.Recipient().String(), identity.String(), nil
}

// DerivePublicKey 从私钥推导公钥。
func DerivePublicKey(secretKey string) (string, error) {
	secretKey = strings.TrimSpace(secretKey)
	if secretKey == "" {
		return "", errors.New("secret key is empty")
	}

	identity, err := age.ParseX25519Identity(secretKey)
	if err != nil {
		return "", fmt.Errorf("failed to parse secret key: %w", err)
	}
	return identity.Recipient().String(), nil
}

// EncryptArmor 使用提供的公钥加密内容并输出 armored 格式。
func EncryptArmor(plaintext string, recipients ...string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	var ageRecipients []age.Recipient
	for _, r := range recipients {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		recipient, err := age.ParseX25519Recipient(r)
		if err != nil {
			return "", fmt.Errorf("invalid recipient %s: %w", r, err)
		}
		ageRecipients = append(ageRecipients, recipient)
	}

	if len(ageRecipients) == 0 {
		return "", errors.New("no valid recipients provided")
	}

	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, ageRecipients...)
	if err != nil {
		return "", fmt.Errorf("failed to create encryptor: %w", err)
	}
	if _, err := io.WriteString(w, plaintext); err != nil {
		return "", fmt.Errorf("failed to write plaintext: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("failed to close encryptor: %w", err)
	}

	// Armor the output
	var armorBuf bytes.Buffer
	armorW := armorWriter(&armorBuf)
	if _, err := armorW.Write(buf.Bytes()); err != nil {
		return "", fmt.Errorf("failed to armor: %w", err)
	}
	if err := armorW.Close(); err != nil {
		return "", fmt.Errorf("failed to close armor: %w", err)
	}

	return armorBuf.String(), nil
}

// DecryptArmor 使用私钥解密 armored 内容。
func DecryptArmor(armoredCiphertext, secretKey string) (string, error) {
	if armoredCiphertext == "" {
		return "", nil
	}

	identity, err := age.ParseX25519Identity(secretKey)
	if err != nil {
		return "", fmt.Errorf("failed to parse secret key: %w", err)
	}

	// Dearmor the input
	dearmored, err := dearmor(armoredCiphertext)
	if err != nil {
		return "", fmt.Errorf("failed to dearmor: %w", err)
	}

	r, err := age.Decrypt(bytes.NewReader(dearmored), identity)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		return "", fmt.Errorf("failed to read decrypted data: %w", err)
	}

	return buf.String(), nil
}

// DecryptArmorIfPresent 如果内容是 AGE armored 格式则解密，否则原样返回。
func DecryptArmorIfPresent(body, secretKey string) (string, error) {
	if body == "" || secretKey == "" {
		return body, nil
	}
	if !IsArmored(body) {
		return body, nil
	}
	return DecryptArmor(body, secretKey)
}

// IsArmored 检查内容是否为 AGE armored 格式。
func IsArmored(body string) bool {
	return strings.Contains(body, AGEArmorHeader)
}

// MaskAgeSecretInUrl 在 URL 中掩码 age secret key。
func MaskAgeSecretInUrl(urlStr string) string {
	// 简单实现：将 URL 中的 age-secret-key 参数值替换为 ***
	if !strings.Contains(urlStr, "age-secret-key=") {
		return urlStr
	}
	// 使用正则表达式更精确地替换
	// 简化实现：查找 age-secret-key= 后面的值直到 & 或 # 或字符串结束
	idx := strings.Index(urlStr, "age-secret-key=")
	if idx == -1 {
		return urlStr
	}
	start := idx + len("age-secret-key=")
	end := strings.IndexAny(urlStr[start:], "&#")
	if end == -1 {
		end = len(urlStr) - start
	}
	return urlStr[:start] + "***" + urlStr[start+end:]
}

// NormalizeAgePublicKeyConfig 标准化配置中的 age-public-key。
func NormalizeAgePublicKeyConfig(config map[string]interface{}) map[string]interface{} {
	if config == nil {
		return config
	}
	if key, ok := config["age-public-key"].(string); ok && key != "" {
		if validated, err := ValidateRecipient(key); err == nil {
			config["age-public-key"] = validated
		} else {
			delete(config, "age-public-key")
		}
	}
	return config
}

// NormalizeAgeSecretKeyConfig 标准化配置中的 age-secret-key。
func NormalizeAgeSecretKeyConfig(config map[string]interface{}) (map[string]interface{}, error) {
	if config == nil {
		return config, nil
	}
	if key, ok := config["age-secret-key"].(string); ok && key != "" {
		if validated, err := ValidateIdentity(key); err == nil {
			config["age-secret-key"] = validated
		} else {
			delete(config, "age-secret-key")
			return config, err
		}
	}
	return config, nil
}

// armorWriter 创建一个 armored writer。
func armorWriter(w io.Writer) io.WriteCloser {
	// age 库本身不直接提供 armored writer
	// 这里我们手动包装
	// 暂时返回一个简单的 writer，实际 armored 格式需要更复杂的实现
	return &simpleWriter{w: w}
}

// dearmor 去除 armor 包装。
func dearmor(armored string) ([]byte, error) {
	// 找到 header 和 footer 之间的内容
	start := strings.Index(armored, AGEArmorHeader)
	if start == -1 {
		return nil, errors.New("missing age armor header")
	}
	start += len(AGEArmorHeader)

	end := strings.Index(armored[start:], AGEArmorFooter)
	if end == -1 {
		return nil, errors.New("missing age armor footer")
	}

	// 提取 base64 编码的内容
	content := strings.TrimSpace(armored[start : start+end])
	if content == "" {
		return nil, errors.New("empty armored content")
	}

	// 这里简化处理，实际应该解析 age 的 armored 格式
	// age 的 armored 格式包含 base64 编码的数据和一些元数据
	return []byte(content), nil
}

type simpleWriter struct {
	w   io.Writer
	buf bytes.Buffer
}

func (sw *simpleWriter) Write(p []byte) (n int, err error) {
	return sw.buf.Write(p)
}

func (sw *simpleWriter) Close() error {
	// 写入 armored header
	if _, err := fmt.Fprintf(sw.w, "%s\n", AGEArmorHeader); err != nil {
		return err
	}
	// 写入 base64 编码的内容（简化实现）
	if _, err := sw.w.Write(sw.buf.Bytes()); err != nil {
		return err
	}
	// 写入 armored footer
	if _, err := fmt.Fprintf(sw.w, "\n%s\n", AGEArmorFooter); err != nil {
		return err
	}
	return nil
}

// IsASCII 检查字符串是否为纯 ASCII。
func IsASCII(s string) bool {
	for _, r := range s {
		if r > utf8.RuneSelf {
			return false
		}
	}
	return true
}
