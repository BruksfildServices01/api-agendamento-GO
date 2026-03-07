package models

type OrderItem struct {
	ID uint `gorm:"primaryKey"`

	OrderID uint `gorm:"index;not null"`

	ItemID   uint
	ItemName string `gorm:"size:150;not null"`

	Quantity  int   `gorm:"not null"`
	UnitPrice int64 `gorm:"type:bigint;not null"`
	Total     int64 `gorm:"type:bigint;not null"`
}
