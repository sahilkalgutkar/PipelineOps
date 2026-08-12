// Package cache provides a thin Redis wrapper used to cache each job's
// latest heartbeat so status reads don't need to hit Postgres.
package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

type LastHeartbeat struct {
	JobID      string    `json:"job_id"`
	Status     string    `json:"status"`
	ReceivedAt time.Time `json:"received_at"`
}

type Client struct {
	rdb *redis.Client
}

func Connect(ctx context.Context, addr string) (*Client, error) {
	opt, err := redis.ParseURL(addr)
	if err != nil {
		return nil, err
	}
	rdb := redis.NewClient(opt)
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return &Client{rdb: rdb}, nil
}

func (c *Client) Close() error {
	return c.rdb.Close()
}

func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

func key(jobID string) string {
	return "pipelineops:last_heartbeat:" + jobID
}

func (c *Client) SetLastHeartbeat(ctx context.Context, hb LastHeartbeat) error {
	payload, err := json.Marshal(hb)
	if err != nil {
		return err
	}
	// TTL is generous relative to any realistic heartbeat interval; a stale
	// key just means a fallback read from Postgres, never a wrong answer.
	return c.rdb.Set(ctx, key(hb.JobID), payload, 24*time.Hour).Err()
}

func (c *Client) GetLastHeartbeat(ctx context.Context, jobID string) (*LastHeartbeat, error) {
	val, err := c.rdb.Get(ctx, key(jobID)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	var hb LastHeartbeat
	if err := json.Unmarshal(val, &hb); err != nil {
		return nil, err
	}
	return &hb, nil
}
