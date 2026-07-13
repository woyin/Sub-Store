package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port                string
	Host                string
	DataBasePath        string
	BackendMerge        bool
	BackendPrefix       string
	FrontendBackendPath string
	FrontendPath        string
	FrontendPort        string
	FrontendHost        string
	FrontendURL         string
	SyncCron            string
	ProduceCron         string
	DownloadCron        string
	UploadCron          string
	MMDBCron            string
	MMDBCountryPath     string
	MMDBCountryURL      string
	MMDBASNPath         string
	MMDBASNURL          string
	DefaultProxy        string
	DefaultUserAgent    string
	DefaultTimeout      int
	CacheThreshold      int
	BodyJSONLimit       string
	XPoweredBy          string
	MaxHeaderSize       int
	PushService         string
	CORSAllowedOrigins  string
}

func Load() *Config {
	defaultTimeout, _ := strconv.Atoi(GetEnv("SUB_STORE_DEFAULT_TIMEOUT", "8000"))
	cacheThreshold, _ := strconv.Atoi(GetEnv("SUB_STORE_CACHE_THRESHOLD", "1024"))
	maxHeaderSize, _ := strconv.Atoi(GetEnv("SUB_STORE_MAX_HEADER_SIZE", "32768"))

	return &Config{
		Port:                GetEnv("SUB_STORE_BACKEND_API_PORT", "3000"),
		Host:                GetEnv("SUB_STORE_BACKEND_API_HOST", "::"),
		DataBasePath:        GetEnv("SUB_STORE_DATA_BASE_PATH", "."),
		BackendMerge:        GetEnv("SUB_STORE_BACKEND_MERGE", "") == "true",
		BackendPrefix:       GetEnv("SUB_STORE_BACKEND_PREFIX", ""),
		FrontendBackendPath: GetEnv("SUB_STORE_FRONTEND_BACKEND_PATH", "/api"),
		FrontendPath:        GetEnv("SUB_STORE_FRONTEND_PATH", ""),
		FrontendPort:        GetEnv("SUB_STORE_FRONTEND_PORT", "3001"),
		FrontendHost:        GetEnv("SUB_STORE_FRONTEND_HOST", "::"),
		FrontendURL:         GetEnv("SUB_STORE_FRONTEND_URL", "https://sub-store.vercel.app"),
		SyncCron:            GetEnv("SUB_STORE_BACKEND_SYNC_CRON", ""),
		ProduceCron:         GetEnv("SUB_STORE_PRODUCE_CRON", ""),
		DownloadCron:        GetEnv("SUB_STORE_BACKEND_DOWNLOAD_CRON", ""),
		UploadCron:          GetEnv("SUB_STORE_BACKEND_UPLOAD_CRON", ""),
		MMDBCron:            GetEnv("SUB_STORE_MMDB_CRON", ""),
		MMDBCountryPath:     GetEnv("SUB_STORE_MMDB_COUNTRY_PATH", ""),
		MMDBCountryURL:      GetEnv("SUB_STORE_MMDB_COUNTRY_URL", ""),
		MMDBASNPath:         GetEnv("SUB_STORE_MMDB_ASN_PATH", ""),
		MMDBASNURL:          GetEnv("SUB_STORE_MMDB_ASN_URL", ""),
		DefaultProxy:        GetEnv("SUB_STORE_BACKEND_DEFAULT_PROXY", ""),
		DefaultUserAgent:    GetEnv("SUB_STORE_DEFAULT_USER_AGENT", "clash.meta/v1.19.23"),
		DefaultTimeout:      defaultTimeout,
		CacheThreshold:      cacheThreshold,
		BodyJSONLimit:       GetEnv("SUB_STORE_BODY_JSON_LIMIT", "1mb"),
		XPoweredBy:          GetEnv("SUB_STORE_X_POWERED_BY", "Sub-Store"),
		MaxHeaderSize:       maxHeaderSize,
		PushService:         GetEnv("SUB_STORE_PUSH_SERVICE", ""),
		CORSAllowedOrigins:  GetEnv("SUB_STORE_CORS_ALLOWED_ORIGINS", ""),
	}
}

func GetEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
