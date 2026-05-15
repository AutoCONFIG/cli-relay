package db

type User struct {
	Base
	Email        string `gorm:"size:255;uniqueIndex;not null" json:"email"`
	Username     string `gorm:"size:100;uniqueIndex;not null" json:"username"`
	PasswordHash string `gorm:"size:255;not null" json:"-"`
	Status       string `gorm:"size:20;default:active" json:"status"` // active, disabled
	Balance      int64  `gorm:"default:0" json:"balance"`              // balance (token units)
}

func (User) TableName() string { return "users" }
