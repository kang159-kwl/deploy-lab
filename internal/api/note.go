package api

import (
	"deploy-lab/internal/db"
	"deploy-lab/internal/model"
	"net/http"

	"github.com/gin-gonic/gin"
)

// CreateNote 创建便签
// 练习目标：
//   - 用 c.ShouldBindJSON 读取请求体
//   - 验证必填字段（title 不能为空）
//   - 用 db.DB.Create() 插入数据
//   - 返回创建后的完整对象（包括自动生成的 ID 和时间戳）
func CreateNote(c *gin.Context) {
	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}

	// TODO 1: 用 c.ShouldBindJSON(&req) 读取请求体
	// 如果失败（JSON 格式错误），返回 400
	// TODO 1: 用 c.ShouldBindJSON(&req) 读取请求体

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "JSON格式错误"})
	}

	// TODO 2: 验证 title 不能为空
	// 提示: if req.Title == "" { c.JSON(400, ...) }
	if req.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "标题不能为空"})
		return
	}
	// TODO 3: 创建 Note 对象
	// 提示: note := model.Note{Title: req.Title, Content: req.Content}
	note := model.Note{
		Title:   req.Title,
		Content: req.Content,
	}
	// TODO 4: 调用 db.DB.Create(&note) 插入数据库
	// 如果失败，返回 500
	// 提示: if err := db.DB.Create(&note).Error; err != nil { ... }
	if err := db.DB.Create(&note).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "创建失败"})
		return
	}
	// TODO 5: 返回 201 Created 和创建后的 note 对象
	// 提示: c.JSON(http.StatusCreated, note)
	c.JSON(http.StatusCreated, note)
}
