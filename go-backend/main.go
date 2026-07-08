package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
)

// Config holds all application configuration from environment variables
type Config struct {
	Port                 string
	Host                 string
	DataBasePath         string
	BackendMerge         bool
	BackendPrefix        string
	FrontendBackendPath  string
	FrontendPath         string
	FrontendPort         string
	FrontendHost         string
	SyncCron             string
	ProduceCron          string
	DownloadCron         string
	UploadCron           string
	MMDBCron             string
	MMDBCountryPath      string
	MMDBCountryURL       string
	MMDBASNPath          string
	MMDBASNURL           string
	DefaultProxy         string
	DefaultUserAgent     string
	DefaultTimeout       int
	CacheThreshold       int
	BodyJSONLimit        string
	XPoweredBy           string
	MaxHeaderSize        int
	PushService          string
	CORSAllowedOrigins   string
}

func loadConfig() *Config {
	defaultTimeout, _ := strconv.Atoi(getEnv("SUB_STORE_DEFAULT_TIMEOUT", "8000"))
	cacheThreshold, _ := strconv.Atoi(getEnv("SUB_STORE_CACHE_THRESHOLD", "1024"))
	maxHeaderSize, _ := strconv.Atoi(getEnv("SUB_STORE_MAX_HEADER_SIZE", "32768"))

	return &Config{
		Port:                getEnv("SUB_STORE_BACKEND_API_PORT", "3000"),
		Host:                getEnv("SUB_STORE_BACKEND_API_HOST", "::"),
		DataBasePath:        getEnv("SUB_STORE_DATA_BASE_PATH", "."),
		BackendMerge:        getEnv("SUB_STORE_BACKEND_MERGE", "") == "true",
		BackendPrefix:       getEnv("SUB_STORE_BACKEND_PREFIX", ""),
		FrontendBackendPath: getEnv("SUB_STORE_FRONTEND_BACKEND_PATH", "/api"),
		FrontendPath:        getEnv("SUB_STORE_FRONTEND_PATH", ""),
		FrontendPort:        getEnv("SUB_STORE_FRONTEND_PORT", "3001"),
		FrontendHost:        getEnv("SUB_STORE_FRONTEND_HOST", "::"),
		SyncCron:            getEnv("SUB_STORE_BACKEND_SYNC_CRON", ""),
		ProduceCron:         getEnv("SUB_STORE_PRODUCE_CRON", ""),
		DownloadCron:        getEnv("SUB_STORE_BACKEND_DOWNLOAD_CRON", ""),
		UploadCron:          getEnv("SUB_STORE_BACKEND_UPLOAD_CRON", ""),
		MMDBCron:            getEnv("SUB_STORE_MMDB_CRON", ""),
		MMDBCountryPath:     getEnv("SUB_STORE_MMDB_COUNTRY_PATH", ""),
		MMDBCountryURL:      getEnv("SUB_STORE_MMDB_COUNTRY_URL", ""),
		MMDBASNPath:         getEnv("SUB_STORE_MMDB_ASN_PATH", ""),
		MMDBASNURL:          getEnv("SUB_STORE_MMDB_ASN_URL", ""),
		DefaultProxy:        getEnv("SUB_STORE_BACKEND_DEFAULT_PROXY", ""),
		DefaultUserAgent:    getEnv("SUB_STORE_DEFAULT_USER_AGENT", "clash.meta/v1.19.23"),
		DefaultTimeout:      defaultTimeout,
		CacheThreshold:      cacheThreshold,
		BodyJSONLimit:       getEnv("SUB_STORE_BODY_JSON_LIMIT", "1mb"),
		XPoweredBy:          getEnv("SUB_STORE_X_POWERED_BY", "Sub-Store"),
		MaxHeaderSize:       maxHeaderSize,
		PushService:         getEnv("SUB_STORE_PUSH_SERVICE", ""),
		CORSAllowedOrigins:  getEnv("SUB_STORE_CORS_ALLOWED_ORIGINS", ""),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func main() {
	cfg := loadConfig()

	fmt.Println(`
┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅
     Sub-Store Go -- v2.36.0-go
┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅┅
`)

	// Initialize data store
	dataDir := filepath.Join(cfg.DataBasePath, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}
	store := NewStore(dataDir)

	// Migrate data if needed
	if err := store.Migrate(); err != nil {
		log.Printf("Migration error: %v", err)
	}

	// Initialize app context
	app := NewApp(cfg, store)

	// Setup router
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware(cfg))
	r.Use(requestLogger())

	// Register all routes
	registerRoutes(r, app)

	// Start cron jobs
	startCronJobs(cfg, app)

	// Start server
	addr := cfg.Host + ":" + cfg.Port
	if cfg.Host == "::" {
		addr = ":" + cfg.Port
	}
	log.Printf("[BACKEND] listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func startCronJobs(cfg *Config, app *App) {
	if cfg.SyncCron == "" && cfg.ProduceCron == "" && cfg.DownloadCron == "" && cfg.UploadCron == "" && cfg.MMDBCron == "" {
		return
	}

	c := cron.New(cron.WithSeconds())

	if cfg.SyncCron != "" {
		_, err := c.AddFunc(cfg.SyncCron, func() {
			log.Printf("[SYNC CRON] %s started", cfg.SyncCron)
			if err := app.SyncArtifacts(); err != nil {
				log.Printf("[SYNC CRON] error: %v", err)
			}
			log.Printf("[SYNC CRON] %s finished", cfg.SyncCron)
		})
		if err != nil {
			log.Printf("[SYNC CRON] failed to schedule: %v", err)
		} else {
			log.Printf("[SYNC CRON] %s enabled", cfg.SyncCron)
		}
	}

	if cfg.ProduceCron != "" {
		// Parse format: "0 */2 * * *,sub,a;0 */3 * * *,col,b"
		// For now, simple cron support
		_, err := c.AddFunc(cfg.ProduceCron, func() {
			log.Printf("[PRODUCE CRON] %s started", cfg.ProduceCron)
			log.Printf("[PRODUCE CRON] %s finished", cfg.ProduceCron)
		})
		if err != nil {
			log.Printf("[PRODUCE CRON] failed to schedule: %v", err)
		} else {
			log.Printf("[PRODUCE CRON] %s enabled", cfg.ProduceCron)
		}
	}

	if cfg.DownloadCron != "" {
		_, err := c.AddFunc(cfg.DownloadCron, func() {
			log.Printf("[DOWNLOAD CRON] %s started", cfg.DownloadCron)
			log.Printf("[DOWNLOAD CRON] %s finished", cfg.DownloadCron)
		})
		if err != nil {
			log.Printf("[DOWNLOAD CRON] failed to schedule: %v", err)
		}
	}

	if cfg.UploadCron != "" {
		_, err := c.AddFunc(cfg.UploadCron, func() {
			log.Printf("[UPLOAD CRON] %s started", cfg.UploadCron)
			log.Printf("[UPLOAD CRON] %s finished", cfg.UploadCron)
		})
		if err != nil {
			log.Printf("[UPLOAD CRON] failed to schedule: %v", err)
		}
	}

	if cfg.MMDBCron != "" && ((cfg.MMDBCountryPath != "" && cfg.MMDBCountryURL != "") || (cfg.MMDBASNPath != "" && cfg.MMDBASNURL != "")) {
		_, err := c.AddFunc(cfg.MMDBCron, func() {
			log.Printf("[MMDB CRON] %s started", cfg.MMDBCron)
			app.DownloadMMDB()
			log.Printf("[MMDB CRON] %s finished", cfg.MMDBCron)
		})
		if err != nil {
			log.Printf("[MMDB CRON] failed to schedule: %v", err)
		} else {
			log.Printf("[MMDB CRON] %s enabled", cfg.MMDBCron)
		}
	}

	c.Start()
}

func corsMiddleware(cfg *Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		allowed := false

		if cfg.CORSAllowedOrigins == "*" || cfg.CORSAllowedOrigins == "" {
			allowed = true
		} else if origin != "" {
			// Simple check, could be more sophisticated
			allowed = true
		}

		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
			if origin == "" {
				c.Header("Access-Control-Allow-Origin", "*")
			}
		}
		c.Header("Access-Control-Allow-Methods", "POST,GET,OPTIONS,PATCH,PUT,DELETE")
		c.Header("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")
		c.Header("X-Powered-By", cfg.XPoweredBy)

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(200)
			return
		}

		c.Next()
	}
}

func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()

		if raw != "" {
			path = path + "?" + raw
		}

		log.Printf("[GIN] %v | %3d | %13v | %15s | %-7s %s",
			start.Format("2006/01/02 - 15:04:05"),
			statusCode,
			latency,
			clientIP,
			method,
			path,
		)
	}
}
