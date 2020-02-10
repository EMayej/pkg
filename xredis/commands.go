package xredis

import (
	"fmt"

	"github.com/go-redis/redis"
	"github.com/pkg/errors"
)

var ErrEmptyHash = fmt.Errorf("the hash stored at key is empty")

// LoadStringKeys loads keys of string of Redis.
func LoadStringKeys(db *redis.Client, keys []string) ([]string, error) {
	rs, err := db.MGet(keys...).Result()
	if err != nil {
		return nil, err
	}

	rv := make([]string, len(keys))
	for i, v := range rs {
		if s, ok := v.(string); ok {
			rv[i] = s
		}
	}

	return rv, nil
}

type EncodeFunc func(map[string]string) ([]byte, error)

// LoadHashKeyEncoded load a single hash key from Redis and encode it with
// encodeFunc.
func LoadHashKeyEncoded(
	db *redis.Client,
	encodeFunc EncodeFunc,
	key string,
) ([]byte, error) {
	rs, err := db.HGetAll(key).Result()
	if err != nil {
		return nil, err
	}
	if len(rs) == 0 {
		return nil, errors.WithMessagef(ErrEmptyHash, "key(%s)", key)
	}

	return encodeFunc(rs)
}

// LoadHashKeysEncoded load hash keys of Redis and encode them with encodeFunc.
func LoadHashKeysEncoded(
	db *redis.Client,
	encodeFunc EncodeFunc,
	keys []string,
) ([][]byte, error) {
	cmds, err := db.Pipelined(func(pipe redis.Pipeliner) error {
		for _, id := range keys {
			pipe.HGetAll(id)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	values := make([][]byte, 0, len(keys))

	for _, cmd := range cmds {
		switch c := cmd.(type) {
		case *redis.StringStringMapCmd:
			// Result() should return no error at all, because the
			// Pipeline returns error if any command fails.
			rs, _ := c.Result()
			if len(rs) == 0 {
				values = append(values, nil)
			} else if v, err := encodeFunc(rs); err == nil {
				values = append(values, v)
			} else {
				return nil, err
			}
		default:
			values = append(values, nil)
		}
	}
	return values, nil
}
