package app

import (
	"fmt"

	"sub-store/internal/config"
	"sub-store/internal/model"
	"sub-store/internal/store"
)

type App struct {
	Config *config.Config
	Store  *store.Store
}

func New(cfg *config.Config, st *store.Store) *App {
	return &App{
		Config: cfg,
		Store:  st,
	}
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

func (a *App) SyncArtifacts() error {
	artifacts := store.GetList[model.Artifact](a.Store, model.ARTIFACTS_KEY)
	for _, art := range artifacts {
		if !art.Sync {
			continue
		}
		a.Info(fmt.Sprintf("Syncing artifact: %s", art.Name))
	}
	return nil
}

func (a *App) ProduceAllArtifacts() {
	artifacts := store.GetList[model.Artifact](a.Store, model.ARTIFACTS_KEY)
	for _, art := range artifacts {
		a.Info(fmt.Sprintf("Producing artifact: %s (type=%s, platform=%s)", art.Name, art.Type, art.Platform))
	}
}

func (a *App) PreFetchSubscriptions() {
	subs := store.GetList[model.Subscription](a.Store, model.SUBS_KEY)
	for _, sub := range subs {
		a.Info(fmt.Sprintf("Pre-fetching subscription: %s", sub.Name))
	}
}

func (a *App) DownloadMMDB() {
	if a.Config.MMDBCountryURL != "" && a.Config.MMDBCountryPath != "" {
		a.Info(fmt.Sprintf("[MMDB CRON] downloading %s to %s", a.Config.MMDBCountryURL, a.Config.MMDBCountryPath))
	}
	if a.Config.MMDBASNURL != "" && a.Config.MMDBASNPath != "" {
		a.Info(fmt.Sprintf("[MMDB CRON] downloading %s to %s", a.Config.MMDBASNURL, a.Config.MMDBASNPath))
	}
}
