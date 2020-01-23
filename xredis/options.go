package xredis

import (
	"encoding/json"

	"github.com/go-redis/redis"

	"github.com/EMayej/pkg/xtime"
)

type Options redis.Options

type jsonOptions struct {
	Addr               string         `json:"addr"`
	Password           string         `json:"password"`
	DB                 int            `json:"db"`
	MaxRetries         int            `json:"max_retries"`
	MinRetryBackoff    xtime.Duration `json:"min_retry_backoff"`
	MaxRetryBackoff    xtime.Duration `json:"max_retry_backoff"`
	DialTimeout        xtime.Duration `json:"dial_timeout"`
	ReadTimeout        xtime.Duration `json:"read_timeout"`
	WriteTimeout       xtime.Duration `json:"write_timeout"`
	PoolSize           int            `json:"pool_size"`
	MinIdleConns       int            `json:"min_idle_conns"`
	MaxConnAge         xtime.Duration `json:"max_conn_age"`
	PoolTimeout        xtime.Duration `json:"pool_timeout"`
	IdleTimeout        xtime.Duration `json:"idle_timeout"`
	IdleCheckFrequency xtime.Duration `json:"idle_check_frequency"`
}

func (o Options) MarshalJSON() ([]byte, error) {
	return json.Marshal(&jsonOptions{
		Addr:               o.Addr,
		Password:           o.Password,
		DB:                 o.DB,
		MaxRetries:         o.MaxRetries,
		MinRetryBackoff:    xtime.Duration(o.MinRetryBackoff),
		MaxRetryBackoff:    xtime.Duration(o.MaxRetryBackoff),
		DialTimeout:        xtime.Duration(o.DialTimeout),
		ReadTimeout:        xtime.Duration(o.ReadTimeout),
		WriteTimeout:       xtime.Duration(o.WriteTimeout),
		PoolSize:           o.PoolSize,
		MinIdleConns:       o.MinIdleConns,
		MaxConnAge:         xtime.Duration(o.MaxConnAge),
		PoolTimeout:        xtime.Duration(o.PoolTimeout),
		IdleTimeout:        xtime.Duration(o.IdleTimeout),
		IdleCheckFrequency: xtime.Duration(o.IdleCheckFrequency),
	})
}
