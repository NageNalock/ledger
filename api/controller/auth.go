package controller

import (
	"errors"
	"fmt"
	"ledger/api/db"
	"ledger/api/model"
	"ledger/api/utils"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type RegisterInput struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginInput struct {
	Username    string `json:"username" binding:"required"`
	Password    string `json:"password" binding:"required"`
	CaptchaID   string `json:"captcha_id"`
	CaptchaCode string `json:"captcha_code"`
}

type loginFailInfo struct {
	Count     int
	UpdatedAt time.Time
}

type captchaInfo struct {
	Code      string
	ExpiresAt time.Time
	Key       string
}

var loginFailMu sync.Mutex
var loginFails = make(map[string]*loginFailInfo)
var captchaStore = make(map[string]captchaInfo)
var captchaRand = rand.New(rand.NewSource(time.Now().UnixNano()))

func Login(c *gin.Context) {
	var input LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	key := buildLoginKey(c, input.Username)
	if isCaptchaRequired(key) {
		if !validateCaptcha(key, input.CaptchaID, input.CaptchaCode) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Captcha required", "captcha_required": true})
			return
		}
	}

	var user model.User
	if err := db.DB.Where("username = ?", input.Username).First(&user).Error; err != nil {
		failCount := recordLoginFailure(key)
		if failCount >= 2 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials", "captcha_required": true})
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		failCount := recordLoginFailure(key)
		if failCount >= 2 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials", "captcha_required": true})
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	clearLoginFailures(key)

	token, err := utils.GenerateToken(user.ID, user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
			"avatar":   user.Avatar,
		},
	})
}

func Register(c *gin.Context) {
	var input RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	username := strings.TrimSpace(input.Username)
	password := strings.TrimSpace(input.Password)
	if username == "" || password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password required"})
		return
	}
	if len(password) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 8 characters"})
		return
	}

	var count int64
	db.DB.Model(&model.User{}).Where("username = ?", username).Count(&count)
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username already taken"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	user := model.User{Username: username, PasswordHash: string(hashedPassword), Role: "user"}
	if err := db.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	token, err := utils.GenerateToken(user.ID, user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
			"avatar":   user.Avatar,
		},
	})
}

func GetCaptcha(c *gin.Context) {
	username := c.Query("username")
	if strings.TrimSpace(username) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username required"})
		return
	}

	key := buildLoginKey(c, username)
	if !isCaptchaRequired(key) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Captcha not required"})
		return
	}

	captchaID, captchaText, captchaCode := generateCaptcha()
	loginFailMu.Lock()
	captchaStore[captchaID] = captchaInfo{
		Code:      captchaCode,
		ExpiresAt: time.Now().Add(5 * time.Minute),
		Key:       key,
	}
	loginFailMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"captcha_id":   captchaID,
		"captcha_text": captchaText,
	})
}

func buildLoginKey(c *gin.Context, username string) string {
	return strings.TrimSpace(username) + "|" + c.ClientIP()
}

func isCaptchaRequired(key string) bool {
	loginFailMu.Lock()
	defer loginFailMu.Unlock()
	info, ok := loginFails[key]
	if !ok {
		return false
	}
	if time.Since(info.UpdatedAt) > 30*time.Minute {
		delete(loginFails, key)
		return false
	}
	return info.Count >= 2
}

func recordLoginFailure(key string) int {
	loginFailMu.Lock()
	defer loginFailMu.Unlock()
	info, ok := loginFails[key]
	if !ok {
		info = &loginFailInfo{Count: 0, UpdatedAt: time.Now()}
		loginFails[key] = info
	}
	if time.Since(info.UpdatedAt) > 30*time.Minute {
		info.Count = 0
	}
	info.Count++
	info.UpdatedAt = time.Now()
	return info.Count
}

func clearLoginFailures(key string) {
	loginFailMu.Lock()
	delete(loginFails, key)
	for id, item := range captchaStore {
		if item.Key == key {
			delete(captchaStore, id)
		}
	}
	loginFailMu.Unlock()
}

func generateCaptcha() (string, string, string) {
	loginFailMu.Lock()
	a := captchaRand.Intn(9) + 1
	b := captchaRand.Intn(9) + 1
	loginFailMu.Unlock()
	code := strconv.Itoa(a + b)
	text := fmt.Sprintf("%d + %d = ?", a, b)
	id := strconv.FormatInt(time.Now().UnixNano(), 10) + strconv.Itoa(a+b)
	return id, text, code
}

func validateCaptcha(key, id, code string) bool {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(code) == "" {
		return false
	}
	loginFailMu.Lock()
	defer loginFailMu.Unlock()
	item, ok := captchaStore[id]
	if !ok {
		return false
	}
	if item.Key != key {
		return false
	}
	if time.Now().After(item.ExpiresAt) {
		delete(captchaStore, id)
		return false
	}
	if strings.TrimSpace(code) != item.Code {
		return false
	}
	delete(captchaStore, id)
	return true
}

type UpdateProfileInput struct {
	Username    string `json:"username"`
	NewPassword string `json:"new_password"`
	Avatar      string `json:"avatar"`
}

func UpdateUserProfile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var input UpdateProfileInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user model.User
	if err := db.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Update Username
	if input.Username != "" && input.Username != user.Username {
		// Check uniqueness
		var count int64
		db.DB.Model(&model.User{}).Where("username = ? AND id != ?", input.Username, user.ID).Count(&count)
		if count > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Username already taken"})
			return
		}
		user.Username = input.Username
	}

	// Update Password
	if input.NewPassword != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}
		user.PasswordHash = string(hashedPassword)
	}

	// Update Avatar
	if input.Avatar != "" {
		user.Avatar = input.Avatar
	}

	if err := db.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Profile updated successfully", "user": user})
}

func GetSystemSettings(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var user model.User
	if err := db.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if user.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin only"})
		return
	}

	var settings []model.SystemSetting
	db.DB.Find(&settings)

	// Convert to map for easier frontend consumption
	settingMap := make(map[string]string)
	for _, s := range settings {
		k := s.SettingKey
		v := s.Value
		lk := strings.ToLower(k)
		if strings.Contains(lk, "key") || strings.Contains(lk, "secret") || strings.Contains(lk, "token") {
			vv := strings.TrimSpace(v)
			if len(vv) <= 4 {
				settingMap[k] = "******"
			} else {
				settingMap[k] = "******" + vv[len(vv)-4:]
			}
			continue
		}
		settingMap[k] = v
	}

	c.JSON(http.StatusOK, settingMap)
}

func UpdateSystemSettings(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var user model.User
	if err := db.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if user.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin only"})
		return
	}

	var input map[string]string
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	for k, v := range input {
		var setting model.SystemSetting
		err := db.DB.Where("setting_key = ?", k).First(&setting).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// Create
				setting = model.SystemSetting{SettingKey: k, Value: v}
				if err := db.DB.Create(&setting).Error; err != nil {
					// Handle duplicate key error if race condition occurs
					// Try update again or ignore
					// For simplicity, we log error but continue
					// Or we can try to update again if Create fails with constraint violation
				}
			} else {
				// Other DB error
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		} else {
			// Update
			setting.Value = v
			db.DB.Save(&setting)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Settings updated"})
}

// User Management for Admin

func GetUsers(c *gin.Context) {
	// Check Admin
	userID, _ := c.Get("userID")
	var currentUser model.User
	db.DB.First(&currentUser, userID)
	if currentUser.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin only"})
		return
	}

	var users []model.User
	db.DB.Find(&users)
	c.JSON(http.StatusOK, users)
}

func CreateUser(c *gin.Context) {
	// Check Admin
	userID, _ := c.Get("userID")
	var currentUser model.User
	db.DB.First(&currentUser, userID)
	if currentUser.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin only"})
		return
	}

	var input RegisterInput // Reuse struct
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	user := model.User{
		Username:     input.Username,
		PasswordHash: string(hashedPassword),
		Role:         "user", // Default role
	}

	if err := db.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username already exists"})
		return
	}

	c.JSON(http.StatusOK, user)
}

func DeleteUser(c *gin.Context) {
	// Check Admin
	userID, _ := c.Get("userID")
	var currentUser model.User
	db.DB.First(&currentUser, userID)
	if currentUser.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin only"})
		return
	}

	id := c.Param("id")
	if id == "1" || id == strconv.FormatUint(uint64(currentUser.ID), 10) { // Prevent deleting self or root admin? ID 1 is usually admin.
		// Better check by role or ID
	}
	// Check if target is admin?
	var targetUser model.User
	if err := db.DB.First(&targetUser, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if targetUser.Role == "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot delete admin"})
		return
	}

	db.DB.Delete(&targetUser)
	c.JSON(http.StatusOK, gin.H{"message": "User deleted"})
}
