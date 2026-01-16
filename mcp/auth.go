package mcp

import (
	"time"

	"github.com/TruthHun/BookStack/models"
	"github.com/astaxie/beego/orm"
)

// ValidateToken validates the member token and returns member_id
func ValidateToken(token string) (int, error) {
	if token == "" {
		return 0, nil
	}

	mt := models.NewMemberToken()
	t, err := mt.FindByFieldFirst("token", token)
	if err != nil {
		if err == orm.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}

	if !t.IsValid {
		return 0, nil
	}

	// 检查 Token 过期时间
	// 如果 ValidTime 是零值 (0001-01-01)，表示该 Token 永久有效，跳过过期检查。
	// 如果 ValidTime 已设置且早于当前时间，则 Token 已过期。
	if !t.ValidTime.IsZero() && t.ValidTime.Before(time.Now()) {
		return 0, nil
	}

	return t.MemberId, nil
}
