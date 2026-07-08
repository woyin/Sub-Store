package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Subscription handlers
func GetAllSubscriptions(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		subs := getList[Subscription](app.Store, SUBS_KEY)
		success(c, subs)
	}
}

func CreateSubscription(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var sub Subscription
		if err := c.ShouldBindJSON(&sub); err != nil {
			failed(c, err)
			return
		}

		app.Info("Creating subscription: " + sub.Name)
		subs := getList[Subscription](app.Store, SUBS_KEY)
		if findByName(subs, sub.Name) != nil {
			failed(c, fmt.Errorf("subscription %s already exists"), http.StatusConflict)
			return
		}

		insertByPosition(&subs, sub, "bottom")
		saveList(app.Store, SUBS_KEY, subs)
		success(c, sub, http.StatusCreated)
	}
}

func ReplaceSubscriptions(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var subs []Subscription
		if err := c.ShouldBindJSON(&subs); err != nil {
			failed(c, err)
			return
		}
		saveList(app.Store, SUBS_KEY, subs)
		success(c, subs)
	}
}

func GetSubscription(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		raw := c.Query("raw")
		subs := getList[Subscription](app.Store, SUBS_KEY)
		sub := findByName(subs, name)
		if sub == nil {
			failed(c, fmt.Errorf("subscription %s not found"), http.StatusNotFound)
			return
		}

		if raw == "1" || raw == "true" {
			c.Header("Content-Type", "application/json")
			c.Header("Content-Disposition", `attachment; filename="`+fmt.Sprintf("sub-store_subscription_%s_%s.json", name, formatDateTime(time.Now()))+`"`)
			c.JSON(200, sub)
			return
		}
		success(c, sub)
	}
}

func UpdateSubscription(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		var sub Subscription
		if err := c.ShouldBindJSON(&sub); err != nil {
			failed(c, err)
			return
		}

		app.Info("Updating subscription: " + name)
		subs := getList[Subscription](app.Store, SUBS_KEY)
		oldSub := findByName(subs, name)
		if oldSub == nil {
			failed(c, fmt.Errorf("subscription %s not found"), http.StatusNotFound)
			return
		}

		if sub.Name == "" {
			sub.Name = oldSub.Name
		}

		// If name changed, update references in collections, artifacts, and files
		if name != sub.Name {
			cols := getList[Collection](app.Store, COLLECTIONS_KEY)
			for i := range cols {
				for j, s := range cols[i].Subscriptions {
					if s == name {
						cols[i].Subscriptions[j] = sub.Name
					}
				}
			}
			saveList(app.Store, COLLECTIONS_KEY, cols)

			artifacts := getList[Artifact](app.Store, ARTIFACTS_KEY)
			for i := range artifacts {
				if artifacts[i].Type == "subscription" && artifacts[i].Source == name {
					artifacts[i].Source = sub.Name
				}
			}
			saveList(app.Store, ARTIFACTS_KEY, artifacts)

			files := getList[File](app.Store, FILES_KEY)
			for i := range files {
				if files[i].SourceType == "subscription" && files[i].SourceName == name {
					files[i].SourceName = sub.Name
				}
			}
			saveList(app.Store, FILES_KEY, files)
		}

		updateByName(subs, name, sub)
		saveList(app.Store, SUBS_KEY, subs)
		success(c, sub)
	}
}

func DeleteSubscription(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		mode := c.Query("mode")
		app.Info("Deleting subscription: " + name)

		subs := getList[Subscription](app.Store, SUBS_KEY)
		if findByName(subs, name) == nil {
			failed(c, fmt.Errorf("subscription %s not found"), http.StatusNotFound)
			return
		}

		if shouldArchiveDeletion(mode) {
			// Archive the subscription
			sub := findByName(subs, name)
			archive := Archive{
				ID:        createArchiveID(),
				Type:      "sub",
				Name:      name,
				Data:      sub,
				CreatedAt: time.Now().Unix(),
			}
			archives := getList[Archive](app.Store, ARCHIVES_KEY)
			archives = append([]Archive{archive}, archives...)
			saveList(app.Store, ARCHIVES_KEY, archives)
		}

		deleteByName(&subs, name)
		saveList(app.Store, SUBS_KEY, subs)

		// Remove from collections
		cols := getList[Collection](app.Store, COLLECTIONS_KEY)
		for i := range cols {
			var newSubs []string
			for _, s := range cols[i].Subscriptions {
				if s != name {
					newSubs = append(newSubs, s)
				}
			}
			cols[i].Subscriptions = newSubs
		}
		saveList(app.Store, COLLECTIONS_KEY, cols)
		success(c, nil)
	}
}

func GetSubscriptionFlow(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		url := c.Query("url")
		subs := getList[Subscription](app.Store, SUBS_KEY)
		sub := findByName(subs, name)
		if sub == nil {
			failed(c, fmt.Errorf("subscription %s not found"), http.StatusNotFound)
			return
		}

		if sub.Source == "local" {
			if sub.SubUserinfo != "" {
				// Parse and return flow info
				flowInfo := parseFlowHeaders(sub.SubUserinfo)
				success(c, flowInfo)
			} else {
				failed(c, fmt.Errorf("local subscription has no flow information"))
				return
			}
		}

		// Remote subscription flow
		if url == "" {
			url = sub.URL
		}
		if url == "" {
			failed(c, fmt.Errorf("no URL provided for subscription"))
			return
		}

		// Fetch flow headers
		flowHeaders, err := getFlowHeaders(url, sub.Proxy, app.Config)
		if err != nil {
			failed(c, err)
			return
		}

		flowInfo := parseFlowHeaders(flowHeaders)
		success(c, flowInfo)
	}
}

func parseFlowHeaders(headers string) map[string]interface{} {
	// Parse flow headers like "upload=xxx; download=xxx; total=xxx; expire=xxx"
	result := make(map[string]interface{})
	if headers == "" {
		return result
	}

	parts := strings.Split(headers, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			key := strings.TrimSpace(kv[0])
			value := strings.TrimSpace(kv[1])
			result[key] = value
		}
	}
	return result
}

func getFlowHeaders(url, proxy string, cfg *Config) (string, error) {
	// Simplified implementation - in real scenario would make HTTP HEAD request
	if url == "" {
		return "", fmt.Errorf("empty URL")
	}
	return "upload=0; download=0; total=10737418240; expire=0", nil
}
