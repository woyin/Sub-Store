// Package gist 提供 Gist 备份/恢复功能，兼容 Node.js 版本 gist.js 的所有特性。
package gist

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"sub-store/internal/ageutil"
)

// Config Gist 配置
type Config struct {
	GistToken    string
	GitHubProxy  string
	GitHubAPIURL string
	AgeSecretKey string
}

// Client Gist 客户端
type Client struct {
	config     Config
	httpClient *http.Client
}

// NewClient 创建 Gist 客户端
func NewClient(config Config) *Client {
	return &Client{
		config:     config,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// BackupResult 备份结果
type BackupResult struct {
	Success bool   `json:"success"`
	URL     string `json:"url,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Upload 上传备份到 Gist
func (c *Client) Upload(content map[string]interface{}) (*BackupResult, error) {
	if c.config.GistToken == "" {
		return nil, fmt.Errorf("gist token is not configured")
	}

	// 序列化内容
	jsonData, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal content: %w", err)
	}

	backupContent := string(jsonData)

	// 如果需要，使用 AGE 加密
	if c.config.AgeSecretKey != "" {
		encrypted, err := ageutil.EncryptArmor(backupContent, c.config.AgeSecretKey)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt backup: %w", err)
		}
		backupContent = encrypted
	}

	// Base64 编码
	encodedContent := base64.StdEncoding.EncodeToString([]byte(backupContent))

	// 创建/更新 Gist
	result, err := c.createOrUpdateGist(encodedContent)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// Download 从 Gist 下载备份
func (c *Client) Download() (map[string]interface{}, error) {
	if c.config.GistToken == "" {
		return nil, fmt.Errorf("gist token is not configured")
	}

	// 查找 Gist
	content, err := c.findAndDownloadGist()
	if err != nil {
		return nil, err
	}

	// Base64 解码
	decoded, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return nil, fmt.Errorf("failed to decode backup: %w", err)
	}

	backupContent := string(decoded)

	// 如果需要，使用 AGE 解密
	if c.config.AgeSecretKey != "" {
		decrypted, err := ageutil.DecryptArmorIfPresent(backupContent, c.config.AgeSecretKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt backup: %w", err)
		}
		backupContent = decrypted
	}

	// 解析 JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(backupContent), &result); err != nil {
		return nil, fmt.Errorf("failed to parse backup: %w", err)
	}

	return result, nil
}

// getBaseURL 获取 GitHub API 基础 URL
func (c *Client) getBaseURL() string {
	if c.config.GitHubAPIURL != "" {
		return strings.TrimSuffix(c.config.GitHubAPIURL, "/")
	}
	if c.config.GitHubProxy != "" {
		return strings.TrimSuffix(c.config.GitHubProxy, "/")
	}
	return "https://api.github.com"
}

// createOrUpdateGist 创建或更新 Gist
func (c *Client) createOrUpdateGist(content string) (*BackupResult, error) {
	// 查找现有 Gist
	existingID, err := c.findExistingGist()
	if err != nil {
		// 查找失败，尝试创建新的
		return c.createGist(content)
	}

	if existingID != "" {
		// 更新现有 Gist
		return c.updateGist(existingID, content)
	}

	// 创建新的 Gist
	return c.createGist(content)
}

// findExistingGist 查找现有的 Sub-Store 备份 Gist
func (c *Client) findExistingGist() (string, error) {
	url := fmt.Sprintf("%s/gists", c.getBaseURL())
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "token "+c.config.GistToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to list gists: HTTP %d", resp.StatusCode)
	}

	var gists []struct {
		ID          string `json:"id"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&gists); err != nil {
		return "", err
	}

	for _, gist := range gists {
		if gist.Description == "Auto Generated Sub-Store Backup" {
			return gist.ID, nil
		}
	}

	return "", nil
}

// createGist 创建新的 Gist
func (c *Client) createGist(content string) (*BackupResult, error) {
	url := fmt.Sprintf("%s/gists", c.getBaseURL())

	body := map[string]interface{}{
		"description": "Auto Generated Sub-Store Backup",
		"public":      false,
		"files": map[string]interface{}{
			"Sub-Store": map[string]interface{}{
				"content": content,
			},
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "token "+c.config.GistToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create gist: HTTP %d, %s", resp.StatusCode, string(body))
	}

	var result struct {
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &BackupResult{Success: true, URL: result.HTMLURL}, nil
}

// updateGist 更新现有 Gist
func (c *Client) updateGist(gistID, content string) (*BackupResult, error) {
	url := fmt.Sprintf("%s/gists/%s", c.getBaseURL(), gistID)

	body := map[string]interface{}{
		"files": map[string]interface{}{
			"Sub-Store": map[string]interface{}{
				"content": content,
			},
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PATCH", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "token "+c.config.GistToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to update gist: HTTP %d, %s", resp.StatusCode, string(body))
	}

	var result struct {
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &BackupResult{Success: true, URL: result.HTMLURL}, nil
}

// findAndDownloadGist 查找并下载 Gist 内容
func (c *Client) findAndDownloadGist() (string, error) {
	gistID, err := c.findExistingGist()
	if err != nil {
		return "", err
	}
	if gistID == "" {
		return "", fmt.Errorf("no backup gist found")
	}

	url := fmt.Sprintf("%s/gists/%s", c.getBaseURL(), gistID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "token "+c.config.GistToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get gist: HTTP %d", resp.StatusCode)
	}

	var result struct {
		Files map[string]struct {
			Content string `json:"content"`
		} `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	for _, file := range result.Files {
		if file.Content != "" {
			return file.Content, nil
		}
	}

	return "", fmt.Errorf("no content found in gist")
}
