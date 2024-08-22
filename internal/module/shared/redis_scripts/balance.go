package redisscripts

import (
	"fmt"

	"github.com/redis/go-redis/v9"
)

type BalanceCacheField = string

const (
	CacheFieldBalance      BalanceCacheField = "balance"
	CacheFieldLastTime     BalanceCacheField = "last"
	CacheFieldCapacity     BalanceCacheField = "capacity"
	CacheFieldRecoveryRate BalanceCacheField = "rate"
)

func GetBalanceScript() *redis.Script {
	script := fmt.Sprintf(`
						local capacity = math.floor(tonumber(ARGV[1]))
						local rate = tonumber(ARGV[2])
						if (capacity <= 0) then
							return 0
						end
						if (rate <= 0) then
							return 0
						end
                        redis.call('hsetnx', KEYS[1], '%[1]s', capacity)
                        local time = redis.call('time')
                        local now = (time[1] * 1000) + math.floor(time[2] / 1000)
                        local last = tonumber(redis.call('hget', KEYS[1], '%[2]s') or now)
                        if (last > now) then last = now end
                        local recovery = math.floor((now - last) * rate)
                        local residual = math.floor(redis.call('hincrby', KEYS[1], '%[1]s', -1))
                        local current = math.min(capacity - 1, residual + recovery)
                        redis.call('hset', KEYS[1], '%[1]s', current)
                        return current
                    `, CacheFieldBalance, CacheFieldLastTime)
	return redis.NewScript(script)
}
