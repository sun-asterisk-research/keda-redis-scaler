package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type redisConnectionManager struct {
	entries map[string]redisConnectionEntry
	cleanup func()
	logger  *zap.SugaredLogger
}

type redisConnectionEntry struct {
	client *redis.Client
	usedAt time.Time
}

func (e *redisConnectionEntry) getClient() *redis.Client {
	e.usedAt = time.Now()
	return e.client
}

func (e *redisConnectionEntry) close() error {
	return e.client.Close()
}

func (m *redisConnectionManager) getRedisClient(metadata ScalerMetadata) (*redis.Client, error) {
	key := fmt.Sprintf("%s:%s", metadata.fullName, metadata.MetricName)

	if entry, ok := m.entries[key]; ok {
		return entry.getClient(), nil
	}

	opts := &redis.Options{
		Username:     metadata.Username,
		Password:     metadata.Password,
		DB:           metadata.Database,
		MaxIdleConns: 1,
	}

	if metadata.Address != "" {
		opts.Addr = metadata.Address
	} else {
		if metadata.Host == "" {
			return nil, errors.New("either an address or host and port is required")
		}

		opts.Addr = net.JoinHostPort(metadata.Host, metadata.Port)
	}

	if metadata.EnableTLS {
		opts.TLSConfig = &tls.Config{
			InsecureSkipVerify: metadata.UnsafeSSL,
		}
	}

	client := redis.NewClient(opts)

	m.entries[key] = redisConnectionEntry{
		client: client,
		usedAt: time.Now(),
	}

	m.logger.With("name", key).Debug("Created new Redis client")

	return client, nil
}

func (m *redisConnectionManager) runGC(ctx context.Context) {
	timer := time.NewTicker(time.Minute)

	for {
		select {
		case <-timer.C:
			for name, entry := range m.entries {
				if time.Since(entry.usedAt) > 5*time.Minute {
					m.logger.With("name", name).Debug("Closing Redis client")
					if err := entry.close(); err != nil {
						m.logger.Error("Error closing Redis client: ", err)
					}
					delete(m.entries, name)
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func (m *redisConnectionManager) closeAll() {
	for _, entry := range m.entries {
		if err := entry.close(); err != nil {
			m.logger.Error("Error closing Redis client: ", err)
		}
	}
}

func NewRedisConnectionManager(logger *zap.Logger) *redisConnectionManager {
	manager := &redisConnectionManager{
		entries: make(map[string]redisConnectionEntry),
		logger:  logger.Sugar().With("component", "connection-manager"),
	}

	ctx, cancel := context.WithCancel(context.Background())
	go manager.runGC(ctx)

	manager.cleanup = func() {
		cancel()
		manager.closeAll()
	}

	return manager
}
