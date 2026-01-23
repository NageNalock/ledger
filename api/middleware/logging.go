package middleware

import (
	"log"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-Id")
		if requestID == "" {
			requestID = uuid.NewString()
		}
		c.Set("requestID", requestID)
		c.Writer.Header().Set("X-Request-Id", requestID)

		start := time.Now()
		userID, _ := c.Get("userID")
		log.Printf("request start id=%s method=%s path=%s user_id=%v ip=%s", requestID, c.Request.Method, c.Request.URL.Path, userID, c.ClientIP())
		c.Next()
		latency := time.Since(start)
		status := c.Writer.Status()
		errMsg := ""
		if len(c.Errors) > 0 {
			errMsg = c.Errors.String()
			log.Printf("request error id=%s status=%d user_id=%v errors=%s\n%s", requestID, status, userID, errMsg, string(debug.Stack()))
		}
		log.Printf("request end id=%s status=%d latency=%s user_id=%v errors=%s", requestID, status, latency, userID, errMsg)
	}
}

func RecoveryWithStack() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				requestID, _ := c.Get("requestID")
				log.Printf("panic id=%v method=%s path=%s err=%v\n%s", requestID, c.Request.Method, c.Request.URL.Path, err, string(debug.Stack()))
				c.AbortWithStatusJSON(500, gin.H{"error": "Internal Server Error"})
			}
		}()
		c.Next()
	}
}
