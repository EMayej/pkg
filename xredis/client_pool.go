package xredis

import (
	"encoding/json"
	"fmt"
	"net"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/EMayej/pkg/xerror"
	"github.com/EMayej/pkg/xtime"
)

const (
	defaultCheckInterval = 5 * time.Second
)

var (
	ErrNoClient = fmt.Errorf("no client available")
)

type Mirrors = []Options

// Config is the configuration for a client pool
type Config struct {
	// The check removes invalid clients and creates new clients for new
	// Redis server. It also updates connection pool status of each client
	// for Prometheus monitoring if enabled.
	//
	// Set this check interval to a reasonable value so invalid clients get
	// removed in time and the connection pool of a client won't be zero if
	// there is no traffic. This means the CheckInterval should be less than
	// the IdleTimeout or MaxConnAge of any connection pool. The default
	// Value is 5 seconds.
	CheckInterval xtime.Duration `json:"check_interval"`

	// a dedicated client will be created for each Redis server behind the
	// same DNS name. Normally a Mirror is a normal redis.Option but the
	// Addr option is set to dns_name:port
	Mirrors `json:"mirrors"`
}

// DecodeHook is a helper function to wrap StringToMirrorsHookFunc for viper. It
// should be used with viper.Unmarshal to translate environment variable to
// []*redis.Options.
func DecodeHook(c *mapstructure.DecoderConfig) {
	if c.DecodeHook == nil {
		c.DecodeHook = StringToMirrorsHookFunc()
	} else {
		c.DecodeHook = mapstructure.ComposeDecodeHookFunc(
			StringToMirrorsHookFunc(),
			c.DecodeHook,
		)
	}
}

// StringToMirrorsHookFunc is a helper function for mapstructure to turn string
// into []*redis.Options, it could be used independently with mapstructure, but
// normally use DecodeHook above with viper.Unmarshal is more convenient
func StringToMirrorsHookFunc() mapstructure.DecodeHookFunc {
	return func(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
		if f.Kind() != reflect.String ||
			t != reflect.TypeOf(Mirrors{}) {
			return data, nil
		}

		var dst []*redis.Options
		err := json.Unmarshal([]byte(data.(string)), &dst)
		return dst, err
	}
}

// Option is a helper type to allow customized some options of the client pool
type Option func(*ClientPool)

// WithLogger sets the logger of the client pool
func WithLogger(logger *zap.Logger) Option {
	if logger == nil {
		panic("nil logger")
	}

	return func(c *ClientPool) {
		c.logger = logger
	}
}

// WithRequestDuration sets requestDuration metric of the client pool.
// requestDuration should has one label left
func WithRequestDuration(requestDuration prometheus.ObserverVec) Option {
	return func(c *ClientPool) {
		c.requestDuration = requestDuration
	}
}

// WithPoolStats sets poolStats metric of the client pool. pool Stats wants two
// labels
func WithPoolStats(poolStats *prometheus.GaugeVec) Option {
	return func(c *ClientPool) {
		c.poolStats = poolStats
	}
}

// ClientPool resolves DNS name to IPs and creates a redis.Client for each
// IP:Port combination. A validation check is triggered regularly to ensure
// crashed Redis server will be removed and new IP:Port will be added.
type ClientPool struct {
	*Config

	logger          *zap.Logger
	requestDuration prometheus.ObserverVec
	poolStats       *prometheus.GaugeVec

	sync.Mutex
	next    int
	clients []*redis.Client

	_closed uint32 // atomic
}

func (p *ClientPool) getClients() []*redis.Client {
	p.Lock()
	defer p.Unlock()

	return p.clients
}

func (p *ClientPool) resetClients(clients []*redis.Client) {
	p.Lock()
	defer p.Unlock()

	p.clients = clients
	p.next = 0
	return
}

// Healthy returns true if current client pool is healthy, it relies on regulate
// function to close invalid clients.
func (p *ClientPool) Healthy() bool {
	if p.closed() {
		return false
	}

	return len(p.getClients()) > 0
}

// realizeAddrs returns IP:Port for Host:Port
func realizeAddrs(hostPort string) ([]string, error) {
	fields := strings.Split(hostPort, ":")
	if len(fields) != 2 {
		return nil, errors.Wrap(xerror.ErrArgs, "invalid hostPort")
	}

	ips, err := net.LookupIP(fields[0])
	if err != nil {
		return nil, err
	}

	var rv []string
	for _, ip := range ips {
		rv = append(rv, fmt.Sprintf("%s:%s", ip, fields[1]))
	}
	return rv, nil
}

// validateClient returns error if a client no longer connects to a Redis.
func validateClient(client *redis.Client) error {
	// Always send Ping command to ensure the client remains valid when
	// there is no business traffic at all.
	_, err := client.Ping().Result()

	if client.PoolStats().TotalConns > 0 {
		return nil
	}

	if err != nil {
		// We hit here only when TotalConns is zero and Ping command
		// failed
		return err
	}
	return nil
}

// populateClients tries to close invalid clients and create clients for new
// Redis server. It returns true if clients have been modified.
//
// Note that the overall poolSize shrinks if some server is down because we
// can't change the pool size of a created client.
func (p *ClientPool) populateClients() bool {
	oldNodes := make(map[string]bool)
	var rogueClients []*redis.Client
	var validClients []*redis.Client
	for _, client := range p.getClients() {
		oldNodes[client.Options().Addr] = true
		if err := validateClient(client); err != nil {
			rogueClients = append(rogueClients, client)
		} else {
			validClients = append(validClients, client)
		}
	}

	for _, mirror := range p.Mirrors {
		ipPorts, err := realizeAddrs(mirror.Addr)
		if err != nil || len(ipPorts) == 0 {
			if p.logger != nil {
				p.logger.Error("xredis: no ips found", zap.String("Addr", mirror.Addr), zap.Error(err))
			}
			continue
		}

		// split poolSize evenly to each IP:Port
		poolSize := mirror.PoolSize / len(ipPorts)
		if poolSize*len(ipPorts) < mirror.PoolSize {
			poolSize++
		}

		for _, ipPort := range ipPorts {
			if oldNodes[ipPort] {
				continue
			}

			optForIP := redis.Options(mirror)
			optForIP.PoolSize = poolSize
			optForIP.Addr = ipPort

			client := redis.NewClient(&optForIP)
			if err := validateClient(client); err != nil {
				if p.logger != nil {
					p.logger.Error("xredis: fail to create new redis client", zap.String("Addr", optForIP.Addr), zap.Error(err))
				}

				// we can't change the pool size of created
				// clients, so a new client gets closed here
				// means the overall pool size shrinks.
				// Meanwhile, if the Redis servers keeps
				// changing, then pool size of each client
				// varies dramatically
				client.Close()

				continue
			}

			if p.requestDuration != nil {
				client.WrapProcess(func(oldProcess func(redis.Cmder) error) func(cmd redis.Cmder) error {
					// Clone ipPort so golang Closure is
					// happy, or otherwise we end up using
					// the last ipPort for all clients
					ipPort := ipPort
					return func(cmd redis.Cmder) error {
						start := time.Now()
						defer func() {
							p.requestDuration.WithLabelValues(ipPort).Observe(time.Since(start).Seconds())
						}()

						return oldProcess(cmd)
					}
				})
				client.WrapProcessPipeline(func(oldProcess func([]redis.Cmder) error) func([]redis.Cmder) error {
					// Make Closure happy
					ipPort := ipPort
					return func(cmds []redis.Cmder) error {
						start := time.Now()
						defer func() {
							p.requestDuration.WithLabelValues(ipPort).Observe(time.Since(start).Seconds())
						}()

						return oldProcess(cmds)
					}
				})
			}

			validClients = append(validClients, client)
		}
	}

	if len(oldNodes) == len(validClients) && len(rogueClients) == 0 {
		return false
	}

	p.resetClients(validClients)

	for _, c := range rogueClients {
		if p.logger != nil {
			p.logger.Error("xredis: closing invalid redis client", zap.String("Addr", c.Options().Addr))
		}

		c.Close()
	}

	return true
}

func (p *ClientPool) updatePoolStats() {
	if p.poolStats == nil {
		return
	}

	for _, client := range p.getClients() {
		stats := client.PoolStats()
		p.poolStats.WithLabelValues(client.Options().Addr, "hits").Set(float64(stats.Hits))
		p.poolStats.WithLabelValues(client.Options().Addr, "misses").Set(float64(stats.Misses))
		p.poolStats.WithLabelValues(client.Options().Addr, "timeouts").Set(float64(stats.Timeouts))
		p.poolStats.WithLabelValues(client.Options().Addr, "total_conns").Set(float64(stats.TotalConns))
		p.poolStats.WithLabelValues(client.Options().Addr, "idle_conns").Set(float64(stats.IdleConns))
		p.poolStats.WithLabelValues(client.Options().Addr, "stale_conns").Set(float64(stats.StaleConns))
	}
}

func (p *ClientPool) regulate(frequency time.Duration) {
	ticker := time.NewTicker(frequency)
	defer ticker.Stop()

	for range ticker.C {
		if p.closed() {
			break
		}

		if p.populateClients() && p.logger != nil {
			p.logger.Info("xredis: change clients")
		}

		p.updatePoolStats()
	}
}

// Next returns the next valid client in client pool, it returns nil if no valid
// client is available
func (p *ClientPool) Next() (*redis.Client, error) {
	p.Lock()
	defer p.Unlock()

	totalClientsNum := len(p.clients)

	if totalClientsNum == 0 {
		return nil, ErrNoClient
	}

	for try := 0; try < totalClientsNum; try++ {
		client := p.clients[p.next]
		p.next = (p.next + 1) % totalClientsNum

		// If a Redis server is down, the client will be used but all
		// the connections should be closed after that. If we hit the
		// same client again before ClientPool.regulate closes it, the
		// TotalConns of the connection pool should be zero. In that
		// case, the client is invalid but we'll leave it for the
		// regulate function to close.
		if client.PoolStats().TotalConns == 0 {
			continue
		}

		return client, nil
	}

	// all clients are invalid
	return nil, ErrNoClient
}

func (p *ClientPool) closed() bool {
	return atomic.LoadUint32(&p._closed) == 1
}

// Close will close all redis.Clients, it should be called only when ClientPool
// is no longer used. It is the user's responsibility to ensure no one is using
// ClientPool or Client of ClientPool when calling Close.
func (p *ClientPool) Close() error {
	if !atomic.CompareAndSwapUint32(&p._closed, 0, 1) {
		return fmt.Errorf("xredis: client pool already closed")
	}

	var lastError error
	for _, client := range p.getClients() {
		if e := client.Close(); e != nil {
			lastError = e
		}
	}
	return lastError
}

// NewClientPool creates a new client pool, it panics if no client is available
func NewClientPool(config *Config, opts ...Option) *ClientPool {
	p := &ClientPool{
		Config: config,
	}

	for _, opt := range opts {
		opt(p)
	}

	if !p.populateClients() {
		panic("no available redis")
	}

	if p.CheckInterval == 0 {
		p.CheckInterval = xtime.Duration(defaultCheckInterval)
	}
	go p.regulate(time.Duration(p.CheckInterval))

	return p
}
