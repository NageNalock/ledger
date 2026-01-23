package main

import (
	"ledger/api/db"
	"ledger/api/router"
	"ledger/api/utils"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gopkg.in/natefinch/lumberjack.v2"
)

func main() {
	if strings.TrimSpace(os.Getenv("JWT_SECRET")) == "" {
		_ = godotenv.Load()
	}
	if strings.TrimSpace(os.Getenv("JWT_SECRET")) == "" {
		log.Fatal("JWT_SECRET is required (set via environment variable or .env file)")
	}
	if _, err := utils.GenerateToken(1, "healthcheck"); err != nil {
		log.Fatal("JWT configuration error: ", err)
	}

	logDir := os.Getenv("LOG_DIR")
	if logDir == "" {
		logDir = "logs"
	}
	_ = os.MkdirAll(logDir, 0755)

	maxSize := getEnvInt("LOG_MAX_SIZE_MB", 50)
	maxBackups := getEnvInt("LOG_MAX_BACKUPS", 10)
	maxAge := getEnvInt("LOG_MAX_AGE_DAYS", 14)

	logWriter := &lumberjack.Logger{
		Filename:   filepath.Join(logDir, "app.log"),
		MaxSize:    maxSize,
		MaxBackups: maxBackups,
		MaxAge:     maxAge,
		Compress:   true,
	}
	log.SetOutput(logWriter)
	gin.DefaultWriter = logWriter
	gin.DefaultErrorWriter = logWriter

	db.InitDB()

	r := router.SetupRouter()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	pidFile := strings.TrimSpace(os.Getenv("PID_FILE"))
	if pidFile == "" {
		pidFile = "server.pid"
	}
	if pidFile != "" {
		_ = os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0644)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		if pidFile != "" {
			_ = os.Remove(pidFile)
		}
		os.Exit(0)
	}()

	log.Println("Server starting on :" + port)
	if err := r.Run(":" + port); err != nil {
		if pidFile != "" {
			_ = os.Remove(pidFile)
		}
		log.Fatal("Server failed to start:", err)
	}
}

func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}
