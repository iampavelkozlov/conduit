package profilecache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	profiledomain "conduit/internal/service/profile"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const keyVersion = "profile:v1:"

type Redis struct {
	client *redis.Client
	ttl    time.Duration
}

func New(address, password string, database int, ttl time.Duration) *Redis {
	return &Redis{
		client: redis.NewClient(&redis.Options{Addr: address, Password: password, DB: database}),
		ttl:    ttl,
	}
}

func (r *Redis) Close() error {
	return r.client.Close()
}

func (r *Redis) GetByID(ctx context.Context, id uuid.UUID) (profiledomain.Profile, bool, error) {
	return r.get(ctx, idKey(id))
}

func (r *Redis) GetByUsername(ctx context.Context, username string) (profiledomain.Profile, bool, error) {
	return r.get(ctx, usernameKey(username))
}

func (r *Redis) Set(ctx context.Context, profile profiledomain.Profile) error {
	payload, err := json.Marshal(profile)
	if err != nil {
		return fmt.Errorf("marshal profile cache value: %w", err)
	}
	_, err = r.client.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Set(ctx, idKey(profile.UserID), payload, r.ttl)
		pipe.Set(ctx, usernameKey(profile.Username), payload, r.ttl)
		return nil
	})
	if err != nil {
		return fmt.Errorf("set profile cache value: %w", err)
	}
	return nil
}

func (r *Redis) Delete(ctx context.Context, id uuid.UUID, usernames ...string) error {
	keys := make([]string, 0, len(usernames)+1)
	keys = append(keys, idKey(id))
	seen := map[string]struct{}{keys[0]: {}}
	for _, username := range usernames {
		key := usernameKey(username)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	if err := r.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("delete profile cache values: %w", err)
	}
	return nil
}

func (r *Redis) get(ctx context.Context, key string) (profiledomain.Profile, bool, error) {
	payload, err := r.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return profiledomain.Profile{}, false, nil
	}
	if err != nil {
		return profiledomain.Profile{}, false, fmt.Errorf("get profile cache value: %w", err)
	}
	var profile profiledomain.Profile
	if err := json.Unmarshal(payload, &profile); err != nil {
		return profiledomain.Profile{}, false, fmt.Errorf("unmarshal profile cache value: %w", err)
	}
	return profile, true, nil
}

func idKey(id uuid.UUID) string {
	return keyVersion + "id:" + id.String()
}

func usernameKey(username string) string {
	return keyVersion + "username:" + username
}
