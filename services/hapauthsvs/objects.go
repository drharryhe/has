package hapauthsvs

import (
	"gorm.io/gorm"
	"time"
)

//账密登录用户表
type SvsApAuthUser struct {
	gorm.Model

	ID        int64     `json:"id"`
	User      string    `json:"user" gorm:"size:50;unique;index:user_idx"` //用户名，即账号
	Password  string    `json:"-" gorm:"size:32"`                          //用户密码
	LastLogin string    `json:"last_login" gorm:"size:19"`                 //最后一次登录
	Locked    bool      `json:"locked"`                                    //账号是否被锁定
	Fails     int       `json:"-"`                                         //登录失败次数
	LockTime  time.Time `json:"lock_time"`                                 // 锁定时间
}
