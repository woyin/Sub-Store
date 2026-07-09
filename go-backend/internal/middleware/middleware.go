package middleware

import (
	"log"
	"net/http"
	"time"

	"sub-store/internal/config"
	"sub-store/internal/model"
	"sub-store/internal/store"

	"github.com/gin-gonic/gin"
)

func CORS(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		allowed := false

		if cfg.CORSAllowedOrigins == "*" || cfg.CORSAllowedOrigins == "" {
			allowed = true
		} else if origin != "" {
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

func RequestLogger() gin.HandlerFunc {
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

func ShareTokenAuth(st *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenVal := c.Query("token")
		if tokenVal == "" {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"status": "failed",
				"error":  gin.H{"message": "share token required"},
			})
			return
		}

		tokens := store.GetList[model.Token](st, model.TOKENS_KEY)
		now := time.Now().UnixMilli()

		for i, t := range tokens {
			if t.Token != tokenVal {
				continue
			}
			if t.Exp > 0 && t.Exp <= now {
				c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
					"status": "failed",
					"error":  gin.H{"message": "share token expired"},
				})
				return
			}
			if t.Count > 0 && t.UsedCount >= t.Count {
				c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
					"status": "failed",
					"error":  gin.H{"message": "share token usage limit reached"},
				})
				return
			}
			if t.Count > 0 {
				tokens[i].UsedCount = t.UsedCount + 1
				store.SaveList(st, model.TOKENS_KEY, tokens)
			}
			c.Set("shareToken", &tokens[i])
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"status": "failed",
			"error":  gin.H{"message": "invalid share token"},
		})
	}
}
