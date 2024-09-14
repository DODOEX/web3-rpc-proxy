package schema

import (
	"time"

	"gorm.io/gorm"
)

type Base struct {
	ID        uint64    `gorm:"primaryKey; autoIncrement" json:"id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

type BaseWithUpdate struct {
	Base
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type BaseWithDelete struct {
	BaseWithUpdate
	DeletedAt gorm.DeletedAt `json:"deleted_at"`
}
