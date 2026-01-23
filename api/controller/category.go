package controller

import (
	"ledger/api/db"
	"ledger/api/model"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func GetCategories(c *gin.Context) {
	var l1s []model.CategoryL1
	if err := db.DB.Find(&l1s).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result []gin.H
	for _, l1 := range l1s {
		var l2s []model.CategoryL2
		db.DB.Where("parent_id = ?", l1.ID).Find(&l2s)
		
		subs := []gin.H{}
		for _, l2 := range l2s {
			subs = append(subs, gin.H{
				"id":   l2.ID,
				"name": l2.Name,
				"parent_id": l2.ParentID,
			})
		}
		
		result = append(result, gin.H{
			"id":       l1.ID,
			"name":     l1.Name,
			"children": subs,
		})
	}

	c.JSON(http.StatusOK, result)
}

func CreateCategory(c *gin.Context) {
	var input struct {
		Name     string `json:"name" binding:"required"`
		ParentID *uint  `json:"parent_id"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if input.ParentID == nil || *input.ParentID == 0 {
		// Level 1
		cat := model.CategoryL1{Name: input.Name}
		if err := db.DB.Create(&cat).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, cat)
	} else {
		// Level 2
		cat := model.CategoryL2{Name: input.Name, ParentID: *input.ParentID}
		if err := db.DB.Create(&cat).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, cat)
	}
}

func UpdateCategory(c *gin.Context) {
	var input struct {
		ID       uint   `json:"id" binding:"required"`
		Name     string `json:"name" binding:"required"`
		ParentID *uint  `json:"parent_id"` // If present, it's L2
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if input.ParentID == nil || *input.ParentID == 0 {
		// Level 1
		var cat model.CategoryL1
		if err := db.DB.First(&cat, input.ID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
			return
		}
		cat.Name = input.Name
		if err := db.DB.Save(&cat).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, cat)
	} else {
		// Level 2
		var cat model.CategoryL2
		if err := db.DB.First(&cat, input.ID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
			return
		}
		cat.Name = input.Name
		if err := db.DB.Save(&cat).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, cat)
	}
}

func DeleteCategory(c *gin.Context) {
	idStr := c.Param("id")
	id, _ := strconv.Atoi(idStr)
	level := c.Query("level") // 1 or 2

	// Find Fallback Category "其他支出" -> "未分类"
	var fallbackL1 model.CategoryL1
	var fallbackL2 model.CategoryL2
	
	// Assuming they exist because of seed. If not, we might fail or create.
	// We search by name
	db.DB.Where("name = ?", "其他支出").First(&fallbackL1)
	if fallbackL1.ID == 0 {
		// Create if missing (shouldn't happen with seed)
		fallbackL1 = model.CategoryL1{Name: "其他支出"}
		db.DB.Create(&fallbackL1)
	}
	
	db.DB.Where("name = ? AND parent_id = ?", "未分类", fallbackL1.ID).First(&fallbackL2)
	if fallbackL2.ID == 0 {
		fallbackL2 = model.CategoryL2{Name: "未分类", ParentID: fallbackL1.ID}
		db.DB.Create(&fallbackL2)
	}

	// Prevent deleting the fallback category
	if (level == "1" && uint(id) == fallbackL1.ID) || (level == "2" && uint(id) == fallbackL2.ID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete the default fallback category"})
		return
	}

	if level == "1" {
		// Deleting L1
		// 1. Reassign transactions with this L1 to fallback
		db.DB.Model(&model.Transaction{}).Where("category_l1_id = ?", id).Updates(map[string]interface{}{
			"category_l1_id": fallbackL1.ID,
			"category_l2_id": fallbackL2.ID,
		})
		
		// 2. Delete L2s under this L1 (or reassign? usually delete)
		// Transactions under these L2s were already reassigned above (as they also matched L1)
		db.DB.Where("parent_id = ?", id).Delete(&model.CategoryL2{})
		
		// 3. Delete L1
		if err := db.DB.Delete(&model.CategoryL1{}, id).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		// Deleting L2
		// 1. Reassign transactions
		db.DB.Model(&model.Transaction{}).Where("category_l2_id = ?", id).Updates(map[string]interface{}{
			"category_l1_id": fallbackL1.ID,
			"category_l2_id": fallbackL2.ID,
		})
		
		// 2. Delete L2
		if err := db.DB.Delete(&model.CategoryL2{}, id).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}
