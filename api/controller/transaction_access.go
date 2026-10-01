package controller

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"ledger/api/db"
	"ledger/api/model"
	"net/http"
)

// MCP is restricted to the authenticated user's entries, except administrators.
// The existing web UI deliberately retains its shared-ledger view.
func transactionQuery(c *gin.Context) *gorm.DB {
	q := db.DB.WithContext(c.Request.Context())
	if c.GetBool("mcpScoped") && c.GetString("role") != "admin" {
		q = q.Where("transactions.user_id = ?", c.GetUint("userID"))
	}
	return q
}

func GetTransaction(c *gin.Context) {
	var tx model.Transaction
	if err := transactionQuery(c).Preload("User").Preload("CategoryL1").Preload("CategoryL2").Preload("Project").Where("transactions.id = ?", c.Param("id")).First(&tx).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(404, gin.H{"error": "Transaction not found"})
		} else {
			c.JSON(500, gin.H{"error": "Failed to read transaction"})
		}
		return
	}
	c.JSON(200, tx)
}

func validateTransactionReferences(c *gin.Context, input TransactionInput) bool {
	var count int64
	if err := db.DB.Model(&model.CategoryL2{}).Where("id = ? AND parent_id = ?", input.CategoryL2ID, input.CategoryL1ID).Count(&count).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to validate category"})
		return false
	}
	if count != 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Category does not exist or parent does not match"})
		return false
	}
	if input.ProjectID != nil {
		if err := db.DB.Model(&model.Project{}).Where("id = ?", *input.ProjectID).Count(&count).Error; err != nil {
			c.JSON(500, gin.H{"error": "Failed to validate project"})
			return false
		}
		if count != 1 {
			c.JSON(400, gin.H{"error": "Project does not exist"})
			return false
		}
	}
	return true
}
