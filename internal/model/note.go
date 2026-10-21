package model

import "time"

// Note 是便签的数据模型。
// 练习目标：
//   - 理解 gorm 的 tag：`gorm:"primaryKey"` 等
//   - CreatedAt/UpdatedAt 会被 GORM 自动维护
//   - DeletedAt 不为 nil 表示软删除（查询时会自动过滤）
type Note struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Title     string    `gorm:"size:200;not null" json:"title"`
	Content   string    `gorm:"type:text" json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// 软删除：DeletedAt 不为 nil 时表示已删除，GORM 查询会自动过滤
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}
