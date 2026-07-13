package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"

	"sub-store/internal/app"
	"sub-store/internal/config"
	"sub-store/internal/handler"
	"sub-store/internal/store"
)

func setupRouter(a *app.App) *gin.Engine {
	r := gin.Default()
	handler.RegisterRoutes(r, a)
	return r
}

func main() {
	cfg := config.Load()
	st := store.NewStore(cfg.DataBasePath)
	if err := st.Migrate(); err != nil {
		log.Fatalf("Failed to migrate store: %v", err)
	}

	log.Printf("Store loaded successfully")

	// Start MMDB cron
	go func() {
		if cfg.MMDBCron != "" {
			c := cron.New()
			_, err := c.AddFunc(cfg.MMDBCron, func() {
				a := app.New(cfg, st)
				a.DownloadMMDB()
			})
			if err != nil {
				log.Printf("[MMDB CRON] failed to schedule: %v", err)
			} else {
				log.Printf("[MMDB CRON] scheduled: %s", cfg.MMDBCron)
			}
			c.Start()
		}
	}()

	// Wait a bit for MMDB to potentially initialize via the cron handler
	time.Sleep(100 * time.Millisecond)

	a := app.New(cfg, st)

	// Sync artifacts cron
	if cfg.SyncCron != "" {
		c := cron.New()
		_, err := c.AddFunc(cfg.SyncCron, func() {
			log.Printf("[SYNC CRON] %s started", cfg.SyncCron)
			if err := a.SyncArtifacts(); err != nil {
				log.Printf("[SYNC CRON] error: %v", err)
			}
			log.Printf("[SYNC CRON] %s finished", cfg.SyncCron)
		})
		if err != nil {
			log.Printf("[SYNC CRON] failed to schedule: %v", err)
		} else {
			log.Printf("[SYNC CRON] scheduled: %s", cfg.SyncCron)
		}
		c.Start()
	}

	// Produce artifacts cron
	if cfg.ProduceCron != "" {
		c := cron.New()
		_, err := c.AddFunc(cfg.ProduceCron, func() {
			log.Printf("[PRODUCE CRON] %s started", cfg.ProduceCron)
			a.ProduceAllArtifacts()
			log.Printf("[PRODUCE CRON] %s finished", cfg.ProduceCron)
		})
		if err != nil {
			log.Printf("[PRODUCE CRON] failed to schedule: %v", err)
		} else {
			log.Printf("[PRODUCE CRON] scheduled: %s", cfg.ProduceCron)
		}
		c.Start()
	}

	// Download cron (pre-fetch subscriptions)
	if cfg.DownloadCron != "" {
		c := cron.New()
		_, err := c.AddFunc(cfg.DownloadCron, func() {
			log.Printf("[DOWNLOAD CRON] %s started", cfg.DownloadCron)
			a.PreFetchSubscriptions()
			log.Printf("[DOWNLOAD CRON] %s finished", cfg.DownloadCron)
		})
		if err != nil {
			log.Printf("[DOWNLOAD CRON] failed to schedule: %v", err)
		} else {
			log.Printf("[DOWNLOAD CRON] scheduled: %s", cfg.DownloadCron)
		}
		c.Start()
	}

	// Upload cron (sync artifacts to Gist)
	if cfg.UploadCron != "" {
		c := cron.New()
		_, err := c.AddFunc(cfg.UploadCron, func() {
			log.Printf("[UPLOAD CRON] %s started", cfg.UploadCron)
			if err := a.SyncArtifacts(); err != nil {
				log.Printf("[UPLOAD CRON] error: %v", err)
			}
			log.Printf("[UPLOAD CRON] %s finished", cfg.UploadCron)
		})
		if err != nil {
			log.Printf("[UPLOAD CRON] failed to schedule: %v", err)
		} else {
			log.Printf("[UPLOAD CRON] scheduled: %s", cfg.UploadCron)
		}
		c.Start()
	}

	router := setupRouter(a)

	addr := strings.TrimPrefix(cfg.Host, "http://")
	addr = strings.TrimPrefix(addr, "https://")
	if !strings.Contains(addr, ":") && cfg.Port != "" {
		if addr == "" {
			addr = ":" + cfg.Port
		} else {
			addr = net.JoinHostPort(addr, cfg.Port)
		}
	} else if cfg.Port != "" {
		// addr already contains port
		_ = cfg.Port
	}

	log.Printf("Sub-Store Go backend running on %s", addr)

	// Graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sig
		log.Println("Shutting down...")
		if err := st.Save(); err != nil {
			log.Printf("Failed to save store: %v", err)
		}
		os.Exit(0)
	}()

	if err := router.Run(addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func init() {}
