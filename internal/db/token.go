package db

type Token struct {
	Base
	UserID      string `gorm:"size:36;index" json:"user_id"` // associated User
	Name        string `gorm:"size:100;not null" json:"name"`
	Key         string `gorm:"size:100;uniqueIndex;not null" json:"key"`
	Enabled     bool   `gorm:"default:true" json:"enabled"`
	IPWhitelist string `gorm:"type:text" json:"ip_whitelist"`
	Unlimited   bool   `gorm:"default:false" json:"unlimited"`
}

func (Token) TableName() string { return "tokens" }
