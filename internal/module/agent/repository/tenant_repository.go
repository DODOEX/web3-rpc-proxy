package repository

import (
	"context"

	"github.com/DODOEX/web3rpcproxy/internal/database"
	"github.com/DODOEX/web3rpcproxy/internal/database/schema"
)

type ITenantRepository interface {
	GetTenantByToken(ctx context.Context, token string, info *schema.Tenant) error
}

type _TenantRepository struct {
	db *database.Database
}

func NewTenantRepository(db *database.Database) ITenantRepository {
	return &_TenantRepository{
		db: db,
	}
}

func (r *_TenantRepository) GetTenantByToken(ctx context.Context, token string, info *schema.Tenant) error {
	return r.db.DB.WithContext(ctx).Take(info, "token = ?", token).Error
}
