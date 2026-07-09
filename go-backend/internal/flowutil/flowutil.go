package flowutil

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"sub-store/internal/cache"
)

var flowCache = cache.New(1 * time.Minute)

type FlowHeaders struct {
	SubUserInfo     string
	ProfileWebPage  string
	PlanName        string
}

type FlowInfo struct {
	Upload        float64 `json:"upload"`
	Download      float64 `json:"download"`
	Total         float64 `json:"total"`
	Expires       *float64 `json:"expires,omitempty"`
	RemainingDays *int     `json:"remainingDays,omitempty"`
	AppURL        string  `json:"appUrl,omitempty"`
	PlanName      string  `json:"planName,omitempty"`
}

func GetFlowField(headers http.Header) string {
	var sub, webPage, planName string
	for k, vals := range headers {
		lower := strings.ToLower(k)
		if len(vals) == 0 {
			continue
		}
		switch lower {
		case "subscription-userinfo":
			sub = vals[0]
		case "profile-web-page-url":
			webPage = vals[0]
		case "plan-name":
			planName = vals[0]
		}
	}
	var parts []string
	if sub != "" {
		parts = append(parts, sub)
	}
	if webPage != "" {
		parts = append(parts, "app_url="+url.QueryEscape(webPage))
	}
	if planName != "" {
		parts = append(parts, "plan_name="+url.QueryEscape(planName))
	}
	return strings.Join(parts, "; ")
}

func GetFlowHeaders(rawURL, ua string, timeout time.Duration) string {
	if rawURL == "" || (!strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://")) {
		return ""
	}

	parsedURL := strings.Split(rawURL, "\n")[0]
	parsedURL = strings.TrimSpace(parsedURL)

	// strip fragment arguments
	if idx := strings.Index(parsedURL, "#"); idx != -1 {
		parsedURL = parsedURL[:idx]
	}

	if ua == "" {
		ua = "clash.meta/v1.19.23"
	}
	if timeout == 0 {
		timeout = 8 * time.Second
	}

	cacheKey := parsedURL + "|" + ua
	if cached, ok := flowCache.Get(cacheKey); ok {
		return cached
	}

	client := &http.Client{Timeout: timeout}

	flowInfo := tryHeadRequest(client, parsedURL, ua)
	if flowInfo == "" {
		flowInfo = tryGetRequest(client, parsedURL, ua)
	}

	flowInfo = strings.TrimSpace(flowInfo)
	if flowInfo != "" {
		flowCache.Set(cacheKey, flowInfo)
	}
	return flowInfo
}

func tryHeadRequest(client *http.Client, u, ua string) string {
	req, err := http.NewRequest("HEAD", u, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", ua)
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	return GetFlowField(resp.Header)
}

func tryGetRequest(client *http.Client, u, ua string) string {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", ua)
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	return GetFlowField(resp.Header)
}

func NormalizeFlowHeader(flowHeaders string) FlowHeaders {
	result := FlowHeaders{}
	if flowHeaders == "" {
		return result
	}

	seen := make(map[string]bool)
	var subParts []string

	pairs := strings.Split(flowHeaders, ";")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		eqIdx := strings.Index(pair, "=")
		if eqIdx == -1 {
			continue
		}
		key := strings.TrimSpace(pair[:eqIdx])
		encodedValue := strings.TrimSpace(pair[eqIdx+1:])

		if seen[key] {
			continue
		}
		seen[key] = true

		decoded, err := url.QueryUnescape(encodedValue)
		if err != nil {
			decoded = encodedValue
		}

		switch key {
		case "upload", "download", "total":
			num, err := strconv.ParseFloat(decoded, 64)
			if err != nil || !isFinite(num) {
				num = 0
			}
			if num < 0 {
				num = 0
			}
			decoded = fmt.Sprintf("%.0f", num)
		case "expire", "reset_day":
			num, err := strconv.ParseFloat(decoded, 64)
			if err != nil || !isFinite(num) || num <= 0 {
				decoded = ""
			} else {
				decoded = fmt.Sprintf("%.0f", num)
			}
		}

		if key == "app_url" {
			result.ProfileWebPage = decoded
		} else if key == "plan_name" {
			result.PlanName = decoded
		} else if decoded != "" {
			subParts = append(subParts, fmt.Sprintf("%s=%s", key, url.QueryEscape(decoded)))
		}
	}

	result.SubUserInfo = strings.Join(subParts, "; ")
	return result
}

func ParseFlowHeaders(flowHeaders string) *FlowInfo {
	if flowHeaders == "" {
		return nil
	}
	info := &FlowInfo{}

	info.Upload = extractNumericField(flowHeaders, "upload")
	info.Download = extractNumericField(flowHeaders, "download")
	info.Total = extractNumericField(flowHeaders, "total")

	if v, ok := extractOptionalNumericField(flowHeaders, "expire"); ok {
		info.Expires = &v
	}
	if v, ok := extractOptionalNumericField(flowHeaders, "reset_day"); ok {
		days := int(v)
		info.RemainingDays = &days
	}

	if appURL := extractEncodedField(flowHeaders, "app_url"); appURL != "" {
		info.AppURL = appURL
	}
	if planName := extractEncodedField(flowHeaders, "plan_name"); planName != "" {
		info.PlanName = planName
	}

	return info
}

func extractNumericField(s, key string) float64 {
	prefix := key + "="
	idx := strings.Index(s, prefix)
	if idx == -1 {
		return 0
	}
	rest := s[idx+len(prefix):]
	end := strings.IndexAny(rest, "; \t\r\n")
	if end == -1 {
		end = len(rest)
	}
	v, err := strconv.ParseFloat(rest[:end], 64)
	if err != nil {
		return 0
	}
	return v
}

func extractOptionalNumericField(s, key string) (float64, bool) {
	prefix := key + "="
	idx := strings.Index(s, prefix)
	if idx == -1 {
		return 0, false
	}
	rest := s[idx+len(prefix):]
	end := strings.IndexAny(rest, "; \t\r\n")
	if end == -1 {
		end = len(rest)
	}
	v, err := strconv.ParseFloat(rest[:end], 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func extractEncodedField(s, key string) string {
	prefix := key + "="
	idx := strings.Index(s, prefix)
	if idx == -1 {
		return ""
	}
	rest := s[idx+len(prefix):]
	end := strings.IndexAny(rest, "; \t\r\n")
	if end == -1 {
		end = len(rest)
	}
	val := rest[:end]
	decoded, err := url.QueryUnescape(val)
	if err != nil {
		return val
	}
	return decoded
}

func isFinite(f float64) bool {
	return !math.IsNaN(f) && !math.IsInf(f, 0)
}

func MergeFlowHeaders(parts ...string) string {
	var nonEmpty []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	return strings.Join(nonEmpty, "; ")
}

func SetFlowResponseHeaders(w http.Header, flowInfo string) {
	if flowInfo == "" {
		return
	}
	normalized := NormalizeFlowHeader(flowInfo)
	if normalized.SubUserInfo != "" {
		w.Set("subscription-userinfo", normalized.SubUserInfo)
	}
	if normalized.ProfileWebPage != "" {
		w.Set("profile-web-page-url", normalized.ProfileWebPage)
	}
	if normalized.PlanName != "" {
		w.Set("plan-name", normalized.PlanName)
	}
}
