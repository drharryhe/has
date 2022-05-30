package hsessionsvs

import "time"

type SvsSessionToken struct {
	ID       uint      `json:"id"`
	Value    string    `gorm:"size:40;index" json:"value"`
	Agent    string    `gorm:"size:50" json:"agent"`
	IP       string    `gorm:"size:15;index" json:"ip"`
	User     string    `gorm:"size:50;index" json:"user"`
	Validity time.Time `json:"validity"`
}
