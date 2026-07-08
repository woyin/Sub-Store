package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Preprocessor preprocesses raw subscription content.
type Preprocessor interface {
	Name() string
	Test(raw string) bool
	Parse(raw string) string
}

// Base64Preprocessor detects and base64-decodes content.
type Base64Preprocessor struct{}

func (b *Base64Preprocessor) Name() string { return "Base64 Pre-processor" }

var base64Indicators = []string{
	"dm1lc3M",    // vmess
	"c3NyOi8v",   // ssr://
	"c29ja3M6Ly", // socks://
	"dHJvamFu",   // trojan
	"c3M6Ly",     // ss://
	"c3NkOi8v",   // ssd://
	"c2hhZG93",   // shadow
	"aHR0c",      // htt
	"dmxlc3M=",   // vless
	"aHlzdGVyaWEy", // hysteria2
	"aHkyOi8v",   // hy2://
	"d2lyZWd1YXJkOi8v", // wireguard://
	"d2c6Ly8=",   // wg://
	"dHVpYzovLw==", // tuic://
}

func (b *Base64Preprocessor) Test(raw string) bool {
	// Must not look like a protocol line already
	if regexp.MustCompile(`^\w+:\/\/\w+`).MatchString(raw) {
		return false
	}
	for _, k := range base64Indicators {
		if strings.Contains(raw, k) {
			return true
		}
	}
	return false
}

func (b *Base64Preprocessor) Parse(raw string) string {
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		// Try URL-safe base64
		decoded, err = base64.URLEncoding.DecodeString(raw)
		if err != nil {
			return raw
		}
	}
	result := string(decoded)
	// Validate that decoded content looks like proxy lines
	if !regexp.MustCompile(`^\w+(:\/\/|\s*=\s*)\w+`).MatchString(result) {
		return raw
	}
	return result
}

// FallbackBase64Preprocessor is a fallback base64 decoder.
type FallbackBase64Preprocessor struct{}

func (f *FallbackBase64Preprocessor) Name() string { return "Fallback Base64 Pre-processor" }
func (f *FallbackBase64Preprocessor) Test(raw string) bool { return true }
func (f *FallbackBase64Preprocessor) Parse(raw string) string {
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(raw)
		if err != nil {
			return raw
		}
	}
	result := string(decoded)
	if !regexp.MustCompile(`^\w+(:\/\/|\s*=\s*)\w+`).MatchString(result) {
		return raw
	}
	return result
}

// HTMLPreprocessor discards HTML responses.
type HTMLPreprocessor struct{}

func (h *HTMLPreprocessor) Name() string { return "HTML Pre-processor" }
func (h *HTMLPreprocessor) Test(raw string) bool {
	return strings.HasPrefix(strings.TrimSpace(raw), "<!DOCTYPE html>")
}
func (h *HTMLPreprocessor) Parse(raw string) string { return "" }

// ClashPreprocessor extracts the proxies section from a full Clash config.
type ClashPreprocessor struct{}

func (c *ClashPreprocessor) Name() string { return "Clash Pre-processor" }
func (c *ClashPreprocessor) Test(raw string) bool {
	if !strings.Contains(raw, "proxies:") {
		return false
	}
	var root map[string]interface{}
	if err := yaml.Unmarshal([]byte(raw), &root); err != nil {
		return false
	}
	proxies, ok := root["proxies"]
	if !ok {
		return false
	}
	_, ok = proxies.([]interface{})
	return ok
}
func (c *ClashPreprocessor) Parse(raw string) string {
	var root map[string]interface{}
	if err := yaml.Unmarshal([]byte(raw), &root); err != nil {
		return raw
	}
	proxies, ok := root["proxies"].([]interface{})
	if !ok {
		return raw
	}
	var sb strings.Builder
	sb.WriteString("proxies:\n")
	for _, p := range proxies {
		if m, ok := p.(map[string]interface{}); ok {
			data, _ := json.Marshal(m)
			sb.WriteString("  - ")
			sb.Write(data)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// SSDPreprocessor converts ssd:// format to multiple ss:// URIs.
type SSDPreprocessor struct{}

func (s *SSDPreprocessor) Name() string { return "SSD Pre-processor" }
func (s *SSDPreprocessor) Test(raw string) bool {
	return strings.HasPrefix(strings.TrimSpace(raw), "ssd://")
}

func (s *SSDPreprocessor) Parse(raw string) string {
	encoded := strings.TrimPrefix(strings.TrimSpace(raw), "ssd://")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(encoded)
		if err != nil {
			return raw
		}
	}

	var ssdInfo map[string]interface{}
	if err := json.Unmarshal(decoded, &ssdInfo); err != nil {
		return raw
	}

	defaultPort := MapGetInt(ssdInfo, "port")
	defaultMethod := MapGetString(ssdInfo, "encryption")
	defaultPassword := MapGetString(ssdInfo, "password")

	serversRaw, ok := ssdInfo["servers"].([]interface{})
	if !ok {
		return raw
	}

	var results []string
	for i, srv := range serversRaw {
		server, ok := srv.(map[string]interface{})
		if !ok {
			continue
		}
		hostname := MapGetString(server, "server")
		if hostname == "" {
			continue
		}
		port := MapGetInt(server, "port")
		if port == 0 {
			port = defaultPort
		}
		method := MapGetString(server, "encryption")
		if method == "" {
			method = defaultMethod
		}
		password := MapGetString(server, "password")
		if password == "" {
			password = defaultPassword
		}
		tag := MapGetString(server, "remarks")
		if tag == "" {
			tag = fmt.Sprintf("%d", i)
		}

		userinfo := base64.StdEncoding.EncodeToString([]byte(method + ":" + password))
		plugin := ""
		if pluginOpts := MapGetString(server, "plugin_options"); pluginOpts != "" {
			pluginName := MapGetString(server, "plugin")
			plugin = "/?plugin=" + url.QueryEscape(pluginName+";"+pluginOpts)
		}

		uri := fmt.Sprintf("ss://%s@%s:%d%s#%s", userinfo, hostname, port, plugin, tag)
		results = append(results, uri)
	}

	return strings.Join(results, "\n")
}

// FullConfigPreprocessor extracts proxy section from [server_local] or [Proxy] format.
type FullConfigPreprocessor struct{}

func (f *FullConfigPreprocessor) Name() string { return "Full Config Preprocessor" }
func (f *FullConfigPreprocessor) Test(raw string) bool {
	return regexp.MustCompile(`^(\[server_local\]|\[Proxy\])`).MatchString(raw)
}
func (f *FullConfigPreprocessor) Parse(raw string) string {
	re := regexp.MustCompile(`(?im)^\[(?:server_local|Proxy)\]([\s\S]+?)^\[.+?\](?:\r?\n|$)`)
	match := re.FindStringSubmatch(raw)
	if len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	return raw
}

// DefaultPreprocessors is the default ordered list of preprocessors.
var DefaultPreprocessors = []Preprocessor{
	&HTMLPreprocessor{},
	&ClashPreprocessor{},
	&Base64Preprocessor{},
	&SSDPreprocessor{},
	&FullConfigPreprocessor{},
	&FallbackBase64Preprocessor{},
}

// Preprocess runs the raw content through all preprocessors.
func Preprocess(raw string) string {
	for _, pp := range DefaultPreprocessors {
		if pp.Test(raw) {
			raw = pp.Parse(raw)
		}
	}
	return raw
}
