package model

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	SUBS_KEY                   = "subs"
	COLLECTIONS_KEY            = "collections"
	FILES_KEY                  = "files"
	MODULES_KEY                = "modules"
	ARTIFACTS_KEY              = "artifacts"
	RULES_KEY                  = "rules"
	TOKENS_KEY                 = "tokens"
	ARCHIVES_KEY               = "archives"
	SETTINGS_KEY               = "settings"
	ARTIFACT_REPOSITORY_KEY    = "Sub-Store Artifacts Repository"
	GIST_BACKUP_KEY            = "Auto Generated Sub-Store Backup"
	GIST_BACKUP_FILE_NAME      = "Sub-Store"
	RESOURCE_CACHE_KEY         = "#sub-store-cached-resource"
	HEADERS_RESOURCE_CACHE_KEY = "#sub-store-cached-headers-resource"
	SCRIPT_RESOURCE_CACHE_KEY  = "#sub-store-cached-script-resource"
	LOGS_KEY                   = "#sub-store-logs"
	DEFAULT_CACHE_TTL          = 60 * 60 * 1000
	DEFAULT_HEADERS_CACHE_TTL  = 60 * 1000
	DEFAULT_SCRIPT_CACHE_TTL   = 48 * 3600 * 1000
	DEFAULT_LOGS_MAX_COUNT     = 0
)

type Subscription struct {
	Name               string     `json:"name"`
	DisplayName        string     `json:"displayName,omitempty"`
	Source             string     `json:"source"`
	URL                string     `json:"url,omitempty"`
	Content            string     `json:"content,omitempty"`
	UA                 string     `json:"ua,omitempty"`
	Proxy              string     `json:"proxy,omitempty"`
	Process            []Operator `json:"process,omitempty"`
	MergeSources       string     `json:"mergeSources,omitempty"`
	IgnoreFailedRemote string     `json:"ignoreFailedRemoteSub,omitempty"`
	NoCache            bool       `json:"noCache,omitempty"`
	SubUserinfo        string     `json:"subUserinfo,omitempty"`
	PassThroughUA      bool       `json:"passThroughUA,omitempty"`
	Tag                []string   `json:"tag,omitempty"`
}

type Collection struct {
	Name               string     `json:"name"`
	DisplayName        string     `json:"displayName,omitempty"`
	Subscriptions      []string   `json:"subscriptions"`
	SubscriptionTags   []string   `json:"subscriptionTags,omitempty"`
	Process            []Operator `json:"process,omitempty"`
	Proxy              string     `json:"proxy,omitempty"`
	IgnoreFailedRemote string     `json:"ignoreFailedRemoteSub,omitempty"`
	SubUserinfo        string     `json:"subUserinfo,omitempty"`
	FirstSubFlow       *bool      `json:"firstSubFlow,omitempty"`
}

type File struct {
	Name               string     `json:"name"`
	DisplayName        string     `json:"displayName,omitempty"`
	Source             string     `json:"source,omitempty"`
	SourceType         string     `json:"sourceType,omitempty"`
	SourceName         string     `json:"sourceName,omitempty"`
	Content            string     `json:"content,omitempty"`
	URL                string     `json:"url,omitempty"`
	UA                 string     `json:"ua,omitempty"`
	Proxy              string     `json:"proxy,omitempty"`
	Process            []Operator `json:"process,omitempty"`
	MergeSources       string     `json:"mergeSources,omitempty"`
	IgnoreFailedRemote string     `json:"ignoreFailedRemoteFile,omitempty"`
	Mode               string     `json:"mode,omitempty"`
	IncludeUnsupported bool       `json:"includeUnsupportedProxy,omitempty"`
}

type Artifact struct {
	Name               string `json:"name"`
	DisplayName        string `json:"displayName,omitempty"`
	Type               string `json:"type"`
	Source             string `json:"source"`
	Platform           string `json:"platform"`
	Sync               bool   `json:"sync,omitempty"`
	Upload             bool   `json:"upload,omitempty"`
	Updated            int64  `json:"updated,omitempty"`
	URL                string `json:"url,omitempty"`
	IncludeUnsupported bool   `json:"includeUnsupportedProxy,omitempty"`
	PrettyYaml         bool   `json:"prettyYaml,omitempty"`
	Cron               string `json:"cron,omitempty"`
}

type Operator struct {
	Type       string                 `json:"type"`
	Args       map[string]interface{} `json:"args,omitempty"`
	Disabled   bool                   `json:"disabled,omitempty"`
	CustomName string                 `json:"customName,omitempty"`
}

type Token struct {
	Token     string `json:"token"`
	Type      string `json:"type,omitempty"`
	Name      string `json:"name,omitempty"`
	ExpiresIn int64  `json:"expiresIn,omitempty"`
	Exp       int64  `json:"exp,omitempty"`
	Count     int    `json:"count,omitempty"`
	UsedCount int    `json:"usedCount,omitempty"`
}

type Archive struct {
	ID        string      `json:"id"`
	Type      string      `json:"type"`
	Name      string      `json:"name"`
	Data      interface{} `json:"data"`
	CreatedAt int64       `json:"createdAt"`
}

type Module struct {
	Name      string `json:"name"`
	Content   string `json:"content"`
	UpdatedAt int64  `json:"updatedAt,omitempty"`
}

type Settings struct {
	GistToken                         string                 `json:"gistToken,omitempty"`
	GitHubProxy                       string                 `json:"githubProxy,omitempty"`
	GitHubProxyRegex                  string                 `json:"githubProxyRegex,omitempty"`
	APIURL                            string                 `json:"apiURL,omitempty"`
	DefaultProxy                      string                 `json:"defaultProxy,omitempty"`
	DefaultUserAgent                  string                 `json:"defaultUserAgent,omitempty"`
	DefaultTimeout                    int                    `json:"defaultTimeout,omitempty"`
	GitHubAPITimeout                  int                    `json:"githubApiTimeout,omitempty"`
	ArtifactSyncBatchSize             int                    `json:"artifactSyncBatchSize,omitempty"`
	CacheThreshold                    int                    `json:"cacheThreshold,omitempty"`
	BackendRequestConcurrency         int                    `json:"backendRequestConcurrency,omitempty"`
	BackendRequestConcurrencyWaitTime int                    `json:"backendRequestConcurrencyWaitTime,omitempty"`
	AgeSecretKey                      string                 `json:"ageSecretKey,omitempty"`
	ResourceCacheTTL                  int                    `json:"resourceCacheTtl,omitempty"`
	HeadersCacheTTL                   int                    `json:"headersCacheTtl,omitempty"`
	ScriptCacheTTL                    int                    `json:"scriptCacheTtl,omitempty"`
	LogsMaxCount                      int                    `json:"logsMaxCount,omitempty"`
	Appearance                        map[string]interface{} `json:"appearanceSetting,omitempty"`
}

func FormatDateTime(t time.Time) string {
	return t.Format("2006-01-02_15-04-05")
}

func ShouldArchiveDeletion(mode string) bool {
	return mode == "archive"
}

func CreateArchiveID() string {
	return fmt.Sprintf("%d-%s", time.Now().Unix(), RandomString(8))
}

func RandomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}

func MaskAgeSecretInUrl(url string) string {
	return url
}

func NormalizeAgePublicKeyConfig(item interface{}) interface{} {
	return item
}

func NormalizeEditorLanguageConfig(item interface{}) interface{} {
	return item
}

func IsMihomoConfigFile(file File) bool {
	return file.SourceType == "mihomoConfig" || file.SourceType == "mihomoProfile"
}

func NormalizeFileConfig(file File) File {
	if file.SourceType == "mihomoProfile" {
		file.SourceType = "mihomoConfig"
	}
	return file
}

func Base64Encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func Base64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

func IsValidUUID(uuid string) bool {
	return len(uuid) == 36 && strings.Count(uuid, "-") == 4
}

func IsIPv4(ip string) bool {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		if len(part) > 1 && part[0] == '0' {
			return false
		}
		for _, c := range part {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	return true
}

func IsIPv6(ip string) bool {
	return strings.Contains(ip, ":")
}

func IsIP(ip string) bool {
	return IsIPv4(ip) || IsIPv6(ip)
}

func IsValidPortNumber(port string) bool {
	if port == "" {
		return false
	}
	for _, c := range port {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func IsNotBlank(s string) bool {
	return strings.TrimSpace(s) != ""
}

func IsPlainObject(v interface{}) bool {
	_, ok := v.(map[string]interface{})
	return ok
}

func GetRandomPort(portString string) int {
	parts := strings.Split(portString, ",")
	if len(parts) > 0 {
		part := strings.TrimSpace(parts[0])
		if strings.Contains(part, "-") {
			return 0
		}
		if p, err := ParseInt(part); err == nil {
			return p
		}
	}
	return 443
}

func ParseInt(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}

func ExtractName(item interface{}) string {
	jsonData, err := json.Marshal(item)
	if err != nil {
		return ""
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(jsonData, &obj); err != nil {
		return ""
	}
	if n, ok := obj["name"].(string); ok {
		return n
	}
	return ""
}
