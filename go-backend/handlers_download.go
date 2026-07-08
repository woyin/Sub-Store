package main

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Download and produce handlers
func DownloadSubscription(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		target := c.Param("target")
		if target == "" {
			target = c.Query("target")
		}
		if target == "" {
			target = c.Query("platform")
		}
		if target == "" {
			target = "JSON"
		}

		app.Info(fmt.Sprintf("Downloading subscription: %s, target: %s", name, target))

		subs := getList[Subscription](app.Store, SUBS_KEY)
		sub := findByName(subs, name)
		if sub == nil {
			failed(c, fmt.Errorf("subscription %s not found"), http.StatusNotFound)
			return
		}

		// Produce output based on target platform
		output, err := produceArtifact("subscription", name, target, app)
		if err != nil {
			failed(c, err)
			return
		}

		if target == "JSON" {
			c.Header("Content-Type", "application/json; charset=utf-8")
		} else {
			c.Header("Content-Type", "text/plain; charset=utf-8")
		}
		c.String(200, output)
	}
}

func DownloadCollection(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		target := c.Param("target")
		if target == "" {
			target = c.Query("target")
		}
		if target == "" {
			target = c.Query("platform")
		}
		if target == "" {
			target = "JSON"
		}

		app.Info(fmt.Sprintf("Downloading collection: %s, target: %s", name, target))

		cols := getList[Collection](app.Store, COLLECTIONS_KEY)
		col := findByName(cols, name)
		if col == nil {
			failed(c, fmt.Errorf("collection %s not found"), http.StatusNotFound)
			return
		}

		output, err := produceArtifact("collection", name, target, app)
		if err != nil {
			failed(c, err)
			return
		}

		if target == "JSON" {
			c.Header("Content-Type", "application/json; charset=utf-8")
		} else {
			c.Header("Content-Type", "text/plain; charset=utf-8")
		}
		c.String(200, output)
	}
}

func SyncAllArtifacts(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		app.Info("Syncing all artifacts")
		if err := app.SyncArtifacts(); err != nil {
			failed(c, err)
			return
		}
		success(c, nil)
	}
}

func SyncArtifact(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		app.Info("Syncing artifact: " + name)

		artifacts := getList[Artifact](app.Store, ARTIFACTS_KEY)
		artifact := findByName(artifacts, name)
		if artifact == nil {
			failed(c, fmt.Errorf("artifact %s not found"), http.StatusNotFound)
			return
		}

		if !artifact.Sync || artifact.Source == "" {
			failed(c, fmt.Errorf("artifact %s is not configured for sync"))
			return
		}

		// Sync the artifact
		// TODO: Implement actual sync logic
		success(c, artifact)
	}
}

func produceArtifact(artifactType, name, target string, app *App) (string, error) {
	// Simplified implementation
	if artifactType == "subscription" {
		subs := getList[Subscription](app.Store, SUBS_KEY)
		sub := findByName(subs, name)
		if sub == nil {
			return "", fmt.Errorf("subscription %s not found", name)
		}
		return fmt.Sprintf("# Sub-Store: %s\n# Target: %s\n# This is a placeholder output\n", sub.Name, target), nil
	}
	return "", fmt.Errorf("artifact type %s not supported", artifactType)
}
