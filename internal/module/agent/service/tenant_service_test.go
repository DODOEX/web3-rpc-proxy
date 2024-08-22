package service

import (
	"context"
	"encoding/json"
	"log"
	"testing"
	"time"

	"github.com/DODOEX/web3rpcproxy/internal/common"
	"github.com/DODOEX/web3rpcproxy/internal/database/schema"
	"github.com/DODOEX/web3rpcproxy/internal/module/agent/repository"
	"github.com/DODOEX/web3rpcproxy/internal/module/shared"
	"github.com/DODOEX/web3rpcproxy/utils/config"
	"github.com/DODOEX/web3rpcproxy/utils/general/types"
	"github.com/DODOEX/web3rpcproxy/utils/helpers"
	"github.com/go-redis/redismock/v9"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/v2"
	"github.com/rs/zerolog"
	gomock "go.uber.org/mock/gomock"
)

type rdbclient struct{}

func (r *rdbclient) Get(ctx context.Context, key string) (any, error) {
	return nil, nil
}

func (r *rdbclient) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	return nil
}

func (r *rdbclient) Del(ctx context.Context, keys ...string) error {
	return nil
}

func (r *rdbclient) Keys(ctx context.Context, pattern string) ([]string, error) {
	return nil, nil
}

type tenant struct {
	v *schema.Tenant
}

func (t tenant) Matches(x any) bool {
	_, ok := x.(*schema.Tenant)
	if ok {
		x = t.v
	}
	return ok
}

func (t tenant) String() string {
	return "Test Tenant"
}

func newConfig(value map[string]any) *config.Conf {
	k := koanf.New(".")
	conf := &config.Conf{Koanf: k}
	if err := conf.Load(confmap.Provider(value, "."), nil); err != nil {
		log.Fatal(err)
	}
	return conf
}

func createTenant(token string) schema.Tenant {
	tenant := schema.Tenant{
		Name:     "test",
		Token:    token,
		Rate:     1,
		Capacity: 100,
		Base: schema.Base{
			ID: types.Uint64(0),
		},
	}
	return tenant
}

func createTenantService(ctrl1 *gomock.Controller, ctrl2 *gomock.Controller) (*shared.MockScripts, redismock.ClientMock, *repository.MockITenantRepository, TenantService) {
	scripts := shared.NewMockScripts(ctrl1)

	rdb, mock := redismock.NewClientMock()
	rclient := &shared.RedisClient{
		Client: rdb,
	}

	repo := repository.NewMockITenantRepository(ctrl2)

	conf := newConfig(map[string]interface{}{
		"cache.tenants-ttl":  time.Minute,
		"cache.tenants-size": 64,
	})

	tenantService := NewTenantService(
		conf,
		zerolog.Nop(),
		rclient,
		scripts,
		repo,
	)
	return scripts, mock, repo, tenantService
}

func TestAccessHasCache(t *testing.T) {
	ctrl1 := gomock.NewController(t)
	defer ctrl1.Finish()

	ctrl2 := gomock.NewController(t)
	defer ctrl2.Finish()

	token := "abc"
	_tenant := createTenant(token)

	rscriptsmock, rdbmock, repo, tenantService := createTenantService(ctrl1, ctrl2)

	ctx := context.Background()

	repo.EXPECT().GetTenantByToken(ctx, token, tenant{
		v: &_tenant,
	}).Return(nil)

	rscriptsmock.EXPECT().Balance(ctx, "app#:default", 100, 1).Return(int64(1), nil)

	v, _ := json.Marshal(_tenant)
	key := helpers.Concat("app#%s", _tenant.Token)
	rdbmock.ExpectGet(key).SetVal(string(v))
	rdbmock.ExpectGet(key + ":default:last").SetVal("10000")

	app, _ := tenantService.Access(ctx, token, "default")

	if app.TenantInfo.ID != _tenant.ID {
		t.Errorf("expected %d, got %d", _tenant.ID, app.TenantInfo.ID)
	}

	if app.TenantInfo.Name != _tenant.Name {
		t.Errorf("expected %s, got %s", _tenant.Name, app.TenantInfo.Name)
	}

	if app.TenantInfo.Token != token {
		t.Errorf("expected %s, got %s", token, app.TenantInfo.Token)
	}

	if app.TenantInfo.Rate != _tenant.Rate {
		t.Errorf("expected %f, got %f", _tenant.Rate, app.TenantInfo.Rate)
	}

	if app.TenantInfo.Capacity != _tenant.Capacity {
		t.Errorf("expected %f, got %f", _tenant.Capacity, app.TenantInfo.Capacity)
	}

	if app.Balance != 1 {
		t.Errorf("expected %d, got %d", 1, app.Balance)
	}

	if app.LastTime != 0 {
		t.Errorf("expected %d, got %d", 0, app.LastTime)
	}

	if app.Offset != 0 {
		t.Errorf("expected %d, got %d", 0, app.Offset)
	}
}
func TestAccessHasNoCache(t *testing.T) {
	ctrl1 := gomock.NewController(t)
	defer ctrl1.Finish()

	scripts := shared.NewMockScripts(ctrl1)
	scripts.EXPECT().Balance(context.Background(), "app#abc:default", float64(100), float64(1)).Return(int64(1), nil)

	token := "abc"

	info := schema.Tenant{
		Name:     "test",
		Token:    token,
		Rate:     1,
		Capacity: 100,
		Base: schema.Base{
			ID: types.Uint64(0),
		},
	}

	key := helpers.Concat("app#%s", token)

	rdb, mock := redismock.NewClientMock()
	rclient := &shared.RedisClient{
		Client: rdb,
	}
	mock.ExpectGet(key).RedisNil()
	mock.Regexp().ExpectSet(key, `.*`, 7*24*time.Hour).SetVal("OK")
	mock.ExpectGet(key + ":default:last").SetVal("10000")

	ctrl2 := gomock.NewController(t)
	defer ctrl2.Finish()
	repo := repository.NewMockITenantRepository(ctrl2)
	repo.EXPECT().GetTenantByToken(context.Background(), gomock.Eq(token), gomock.Any()).Return(nil).SetArg(1, info)

	tenantService := NewTenantService(
		nil,
		zerolog.Nop(),
		rclient,
		scripts,
		repo,
	)

	app, _ := tenantService.Access(context.Background(), token, "default")

	if app.TenantInfo.ID != info.ID {
		t.Errorf("expected %d, got %d", info.ID, app.TenantInfo.ID)
	}

	if app.TenantInfo.Name != info.Name {
		t.Errorf("expected %s, got %s", info.Name, app.TenantInfo.Name)
	}

	if app.TenantInfo.Token != info.Token {
		t.Errorf("expected %s, got %s", info.Token, app.TenantInfo.Token)
	}

	if app.TenantInfo.Rate != info.Rate {
		t.Errorf("expected %f, got %f", info.Rate, app.TenantInfo.Rate)
	}

	if app.TenantInfo.Capacity != info.Capacity {
		t.Errorf("expected %f, got %f", info.Capacity, app.TenantInfo.Capacity)
	}

	if app.Balance != 1 {
		t.Errorf("expected %d, got %d", 1, app.Balance)
	}

	if app.LastTime != 0 {
		t.Errorf("expected %d, got %d", 0, app.LastTime)
	}

	if app.Offset != 0 {
		t.Errorf("expected %d, got %d", 0, app.Offset)
	}
}

func TestAffected(t *testing.T) {
	ctrl1 := gomock.NewController(t)
	defer ctrl1.Finish()

	scripts := shared.NewMockScripts(ctrl1)

	_app := common.App{
		TenantInfo: schema.Tenant{
			Name:     "test",
			Token:    "abc",
			Rate:     1,
			Capacity: 100,
			Base: schema.Base{
				ID: types.Uint64(0),
			},
		},
		Bucket:  "defatult",
		Balance: 1,
	}

	key := helpers.Concat("app#", _app.Token, ":", _app.Bucket)
	now := int64(1713926158000)

	rdb, mock := redismock.NewClientMock()
	rclient := &shared.RedisClient{
		Client: rdb,
	}
	mock.ExpectHSet(key, "last", now).SetVal(1)

	ctrl2 := gomock.NewController(t)
	defer ctrl2.Finish()
	repo := repository.NewMockITenantRepository(ctrl2)

	tenantService := NewTenantService(
		nil,
		zerolog.Nop(),
		rclient,
		scripts,
		repo,
	)

	err := tenantService.Affected(&_app)

	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}

	if _app.LastTime == 0 {
		t.Errorf("expected now, got %T", _app.LastTime)
	}
}

func TestUnaffected(t *testing.T) {
	ctrl1 := gomock.NewController(t)
	defer ctrl1.Finish()

	scripts := shared.NewMockScripts(ctrl1)

	_app := common.App{
		TenantInfo: schema.Tenant{
			Name:     "test",
			Token:    "abc",
			Rate:     1,
			Capacity: 100,
			Base: schema.Base{
				ID: types.Uint64(0),
			},
		},
		Bucket:  "defatult",
		Balance: 1,
	}

	key := helpers.Concat("app#", _app.Token, ":", _app.Bucket)

	rdb, mock := redismock.NewClientMock()
	rclient := &shared.RedisClient{
		Client: rdb,
	}
	mock.ExpectHIncrBy(key, "balance", 1).SetVal(1)

	ctrl2 := gomock.NewController(t)
	defer ctrl2.Finish()
	repo := repository.NewMockITenantRepository(ctrl2)

	tenantService := NewTenantService(
		nil,
		zerolog.Nop(),
		rclient,
		scripts,
		repo,
	)

	err := tenantService.Unaffected(&_app)

	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}

	if _app.Offset == 0 {
		t.Errorf("expected %d, got %d", 1, _app.Offset)
	}
}

// func TestBatchCall(t *testing.T) {
// }

// func TestSignleCall(t *testing.T) {
// }

// // 允许id为空
// func TestSignleCallIdIsNull(t *testing.T) {
// }

// // 不允许多个id为空
// func TestBatchCallIdIsNull(t *testing.T) {
// [
//     {
//         "jsonrpc": "2.0",
//         "id": null,
//         "method": "net_version",
//         "params": []
//     },
//     {
//         "jsonrpc": "2.0",
//         "method": "eth_blockNumber",
//         "params": [],
//         "id": 2
//     },
//     {
//         "jsonrpc": "2.0",
//         "id": null,
//         "method": "eth_call",
//         "params": [
//             {
//                 "data": "0x8da5cb5b36e7f68c1d2e56001220cdbdd3ba2616072f718acfda4a06441a807d",
//                 "from": "0x0000000000000000000000000000000000000000",
//                 "to": "0x1111111254EEB25477B68fb85Ed929f73A960582"
//             },
//             "latest"
//         ]
//     }
// ]

// 	[
//     {
//         "jsonrpc": "2.0",
//         "error": {
//             "code": -32600,
//             "message": "Invalid Request"
//         },
//         "id": null
//     },
//     {
//         "jsonrpc": "2.0",
//         "error": {
//             "code": -32600,
//             "message": "Invalid Request"
//         },
//         "id": 2
//     },
//     {
//         "jsonrpc": "2.0",
//         "error": {
//             "code": -32600,
//             "message": "Invalid Request"
//         },
//         "id": null
//     }
// ]

// }

// https://goerli.infura.io/v3/9aa3d95b3bc440fa88ea12eaa4456161
// func TestResultIsError() {

// }

// // wss://ws.bitlayer.org
// func TestWebsocketEndpoint() {

// }
