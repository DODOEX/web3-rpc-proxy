package seeds

import (
	"github.com/DODOEX/web3rpcproxy/internal/app/database/schema"
	"github.com/jackc/pgx/pgtype"
	"gorm.io/gorm"
)

type TenantSeeder struct{}

var tenantValues = []schema.Tenant{
	{
		Name:        "测试应用1",
		Token:       "token1",
		Rate:        500.00,
		Capacity:    1000.00,
		Preferences: nil,
	},
	{
		Name:     "测试应用2",
		Token:    "token2",
		Rate:     500.00,
		Capacity: 1000.00,
		Preferences: &pgtype.JSONB{
			Bytes:  []byte(`{"key1": "value1", "key2": "value2"}`),
			Status: pgtype.Present,
		},
	},
}

func (TenantSeeder) Seed(conn *gorm.DB) error {
	for _, value := range tenantValues {
		if err := conn.Create(&value).Error; err != nil {
			return err
		}
	}

	return nil
}

func (TenantSeeder) Count(conn *gorm.DB) (int, error) {
	var count int64
	if err := conn.Model(&schema.Tenant{}).Count(&count).Error; err != nil {
		return 0, err
	}

	return int(count), nil
}
