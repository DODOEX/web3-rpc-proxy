package schema

import "github.com/jackc/pgx/pgtype"

type Tenant struct {
	Name        string        `gorm:"varchar(255); notNull;" json:"name"`
	Token       string        `gorm:"varchar(255); notNull;" json:"token"`
	Rate        float64       `gorm:"type:real; notNull;" json:"rate"` // 每秒释放量
	Capacity    float64       `gorm:"notNull;" json:"capacity"`        // 每秒请求量
	Preferences *pgtype.JSONB `gorm:"type:jsonb; notNull; default:'{}'::jsonb;" json:"preferences"`

	Base
}
