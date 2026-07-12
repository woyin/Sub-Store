package app

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"sub-store/internal/config"
	"sub-store/internal/geoip"
	"sub-store/internal/store"
)

// App 是 Sub-Store 的核心应用结构。
type App struct {
	Config *config.Config
	Store  *store.Store
}

// New 创建一个新的 App 实例。
func New(cfg *config.Config, st *store.Store) *App {
	a := &App{
		Config: cfg,
		Store:  st,
	}

	// Initialize MMDB if paths are configured
	if cfg.MMDBCountryPath != "" || cfg.MMDBASNPath != "" {
		if err := geoip.InitMMDB(cfg.MMDBCountryPath, cfg.MMDBASNPath); err != nil {
			a.Warn(fmt.Sprintf("Failed to initialize MMDB: %v", err))
		} else {
			a.Info("MMDB initialized successfully")
		}
	}

	return a
}

func (a *App) Info(msg string) {
	fmt.Printf("[sub-store] INFO: %s\n", msg)
}

func (a *App) Error(msg string) {
	fmt.Printf("[sub-store] ERROR: %s\n", msg)
}

func (a *App) Warn(msg string) {
	fmt.Printf("[sub-store] WARN: %s\n", msg)
}

func (a *App) Log(msg string) {
	fmt.Printf("[sub-store] LOG: %s\n", msg)
}

func (a *App) Notify(title, subtitle, content string) {
	fmt.Printf("[Notify] %s\n%s\n%s\n\n", title, subtitle, content)
	if a.Config.PushService != "" {
		go a.sendPushNotification(title, subtitle, content)
	}
}

func (a *App) sendPushNotification(title, subtitle, content string) error {
	if a.Config.PushService == "" {
		return nil
	}
	fmt.Printf("[Push Service] URL: %s\n", a.Config.PushService)
	return nil
}

func (a *App) DownloadMMDB() {
	needsReinit := false

	if a.Config.MMDBCountryURL != "" && a.Config.MMDBCountryPath != "" {
		a.Info(fmt.Sprintf("[MMDB CRON] downloading country DB from %s to %s", a.Config.MMDBCountryURL, a.Config.MMDBCountryPath))
		if err := downloadFile(a.Config.MMDBCountryURL, a.Config.MMDBCountryPath); err != nil {
			a.Warn(fmt.Sprintf("[MMDB CRON] failed to download country DB: %v", err))
		} else {
			needsReinit = true
		}
	}
	if a.Config.MMDBASNURL != "" && a.Config.MMDBASNPath != "" {
		a.Info(fmt.Sprintf("[MMDB CRON] downloading ASN DB from %s to %s", a.Config.MMDBASNURL, a.Config.MMDBASNPath))
		if err := downloadFile(a.Config.MMDBASNURL, a.Config.MMDBASNPath); err != nil {
			a.Warn(fmt.Sprintf("[MMDB CRON] failed to download ASN DB: %v", err))
		} else {
			needsReinit = true
		}
	}

	if needsReinit {
		if err := geoip.InitMMDB(a.Config.MMDBCountryPath, a.Config.MMDBASNPath); err != nil {
			a.Warn(fmt.Sprintf("Failed to reinitialize MMDB after download: %v", err))
		} else {
			a.Info("MMDB reinitialized successfully after download")
		}
	}
}

// downloadFile 从 URL 下载文件到本地路径，使用原子写入。
func downloadFile(url, filePath string) error {
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP GET failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	tmpPath := filePath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write file: %w", err)
	}
	f.Close()

	if err := os.Rename(tmpPath, filePath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}
