package xredis

import (
	"encoding/json"

	"github.com/go-redis/redis"

	"github.com/EMayej/pkg/xtime"
)

type ClusterOptions redis.ClusterOptions

type jsonClusterOptions struct {
	Addrs              []string       `json:"addrs"`
	MaxRedirects       int            `json:"max_redirects"`
	ReadOnly           bool           `json:"read_only"`
	RouteByLatency     bool           `json:"route_by_latency"`
	RouteRandomly      bool           `json:"route_randomly"`
	Password           string         `json:"password"`
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

func (o ClusterOptions) MarshalJSON() ([]byte, error) {
	return json.Marshal(&jsonClusterOptions{
		Addrs:              o.Addrs,
		MaxRedirects:       o.MaxRedirects,
		ReadOnly:           o.ReadOnly,
		RouteByLatency:     o.RouteByLatency,
		RouteRandomly:      o.RouteRandomly,
		Password:           o.Password,
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
