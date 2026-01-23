package model

import (
	"time"
)

type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"uniqueIndex;size:50;not null" json:"username"`
	PasswordHash string    `gorm:"size:255;not null" json:"-"`
	Role         string    `gorm:"size:20;default:'user'" json:"role"` // 'admin' or 'user'
	Avatar       string    `gorm:"size:255" json:"avatar"`             // URL path to avatar image
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type SystemSetting struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	SettingKey string    `gorm:"uniqueIndex;size:50;not null;column:setting_key" json:"key"`
	Value      string    `gorm:"size:255" json:"value"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Project struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	Name          string    `gorm:"size:100;not null" json:"name"`
	Description   string    `gorm:"size:255" json:"description"`
	BudgetEnabled bool      `gorm:"default:false" json:"budget_enabled"`
	BudgetLimit   float64   `gorm:"type:decimal(10,2);default:0" json:"budget_limit"`
	CreatedAt     time.Time `json:"created_at"`
}

type CategoryL1 struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:50;not null;uniqueIndex" json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type CategoryL2 struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	ParentID   uint       `gorm:"not null;index" json:"parent_id"`
	Name       string     `gorm:"size:50;not null" json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	CategoryL1 CategoryL1 `gorm:"foreignKey:ParentID" json:"-"`
}

type Transaction struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	UserID          uint      `gorm:"not null;index" json:"user_id"`
	TransactionTime time.Time `gorm:"not null;index" json:"transaction_time"`
	Merchant        string    `gorm:"size:100;index" json:"merchant"`
	Amount          float64   `gorm:"type:decimal(10,2);not null;index" json:"amount"`
	CategoryL1ID    uint      `gorm:"not null;index" json:"category_l1_id"`
	CategoryL2ID    uint      `gorm:"not null;index" json:"category_l2_id"`
	Source          string    `gorm:"size:20" json:"source"`
	Remark          string    `json:"remark"`
	ProjectID       *uint     `gorm:"index" json:"project_id"`                 // Nullable
	ExternalID      string    `gorm:"size:100;uniqueIndex" json:"external_id"` // For import deduplication (uniqueIndex is better)
	CreatedAt       time.Time `json:"created_at"`

	User       User       `gorm:"foreignKey:UserID" json:"user"`
	CategoryL1 CategoryL1 `gorm:"foreignKey:CategoryL1ID" json:"category_l1"`
	CategoryL2 CategoryL2 `gorm:"foreignKey:CategoryL2ID" json:"category_l2"`
	Project    Project    `gorm:"foreignKey:ProjectID" json:"project"`
}

type TransactionDraft struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	UserID          uint      `gorm:"not null;index" json:"user_id"`
	TransactionTime time.Time `json:"transaction_time"`
	Merchant        string    `json:"merchant"`
	Amount          float64   `json:"amount"`
	CategoryL1Name  string    `json:"category_l1_name"`
	CategoryL2Name  string    `json:"category_l2_name"`
	Source          string    `json:"source"`
	Remark          string    `json:"remark"`
	ExternalID      string    `gorm:"index" json:"external_id"`
	ProjectID       *uint     `gorm:"index" json:"project_id"`

	Status   string `gorm:"size:20;default:'pending';index" json:"status"` // pending, analyzing, success, error
	ErrorMsg string `json:"error_msg"`
	RawData  string `json:"raw_data"`

	CreatedAt time.Time `json:"created_at"`
}

type AnalysisRun struct {
	ID               string     `gorm:"primaryKey" json:"id"`
	UserID           uint       `gorm:"not null;index" json:"user_id"`
	ProjectID        *uint      `gorm:"index" json:"project_id"`
	StartDate        string     `gorm:"not null;index" json:"start_date"`
	EndDate          string     `gorm:"not null;index" json:"end_date"`
	Compare          string     `gorm:"size:20;default:'none'" json:"compare"`
	TrendType        string     `gorm:"size:20;default:'day'" json:"trend_type"`
	Language         string     `gorm:"size:20;default:'zh-CN'" json:"language"`
	RequestHash      string     `gorm:"size:64;index" json:"request_hash"`
	RequestJSON      string     `gorm:"type:text" json:"request_json"`
	ResultJSON       string     `gorm:"type:text" json:"result_json"`
	Status           string     `gorm:"size:20;default:'pending';index" json:"status"`
	ErrorMsg         string     `gorm:"type:text" json:"error_msg"`
	StartedAt        *time.Time `gorm:"index" json:"started_at"`
	FinishedAt       *time.Time `gorm:"index" json:"finished_at"`
	DurationMs       *int64     `gorm:"index" json:"duration_ms"`
	PromptLength     int        `gorm:"index" json:"prompt_length"`
	TransactionCount int        `gorm:"index" json:"transaction_count"`
	CreatedAt        time.Time  `gorm:"not null;index" json:"created_at"`
}

type Budget struct {
	ID          string     `gorm:"primaryKey" json:"id"`
	Month       string     `gorm:"size:7;not null;index" json:"month"`
	Status      string     `gorm:"size:20;default:'draft';index" json:"status"`
	Granularity string     `gorm:"size:10;default:'l1'" json:"granularity"`
	TotalAmount float64    `gorm:"type:decimal(12,2);default:0" json:"total_amount"`
	Thresholds  string     `gorm:"type:text" json:"thresholds"`
	DiscardedAt *time.Time `gorm:"index" json:"discarded_at"`
	CreatedAt   time.Time  `gorm:"not null;index" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"not null;index" json:"updated_at"`
}

type BudgetLine struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	BudgetID     string    `gorm:"not null;index" json:"budget_id"`
	CategoryL1ID *uint     `gorm:"index" json:"category_l1_id,omitempty"`
	CategoryL2ID *uint     `gorm:"index" json:"category_l2_id,omitempty"`
	Amount       float64   `gorm:"type:decimal(12,2);default:0" json:"amount"`
	Note         string    `gorm:"size:255" json:"note"`
	CreatedAt    time.Time `gorm:"not null;index" json:"created_at"`
	UpdatedAt    time.Time `gorm:"not null;index" json:"updated_at"`

	CategoryL1 CategoryL1 `gorm:"foreignKey:CategoryL1ID" json:"category_l1"`
	CategoryL2 CategoryL2 `gorm:"foreignKey:CategoryL2ID" json:"category_l2"`
}
