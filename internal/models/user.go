package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// StringSlice persists []string as JSON text in PostgreSQL.
type StringSlice []string

func (s StringSlice) Value() (driver.Value, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (s *StringSlice) Scan(value interface{}) error {
	if value == nil {
		*s = StringSlice{}
		return nil
	}
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return fmt.Errorf("StringSlice: cannot scan type %T", value)
	}
	return json.Unmarshal(b, s)
}

type User struct {
	ID           uint        `gorm:"primaryKey" json:"id"`
	BarbershopID *uint       `json:"barbershop_id"`
	Barbershop   *Barbershop `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`

	Name         string      `gorm:"size:100;not null"`
	Email        string      `gorm:"size:100;uniqueIndex;not null"`
	PasswordHash string      `gorm:"size:255;not null"`
	Phone        string      `gorm:"size:20"`
	Role         UserRole    `gorm:"type:user_role;not null;default:'owner'"`
	SeenTours    StringSlice `gorm:"type:text;not null;default:'[]'" json:"seen_tours"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
