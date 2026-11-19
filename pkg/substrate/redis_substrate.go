package substrate

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RedisCommander defines the subset of Redis operations required by RedisSubstrate.
type RedisCommander interface {
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error)
	Get(ctx context.Context, key string) (string, error)
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error)
	Del(ctx context.Context, keys ...string) (int64, error)
}

// CASLuaScript is the production Redis Lua script executing atomic CAS version checks and mutations.
const CASLuaScript = `
local key = KEYS[1]
local expected_version = tonumber(ARGV[1])
local new_data = ARGV[2]
local ttl_ms = tonumber(ARGV[3])

local current_version_str = redis.call('HGET', key, 'version')
local current_version = 0
if current_version_str then
    current_version = tonumber(current_version_str)
end

if expected_version == 0 then
    if current_version_str then
        return redis.error_reply("CAS_CONFLICT: artifact already exists")
    end
else
    if not current_version_str or current_version ~= expected_version then
        return redis.error_reply("CAS_CONFLICT: version mismatch")
    end
end

local next_version = current_version + 1
redis.call('HSET', key, 'data', new_data, 'version', tostring(next_version))

if ttl_ms > 0 then
    redis.call('PEXPIRE', key, ttl_ms)
end

return tostring(next_version)
`

// GetLuaScript atomically fetches version and payload.
const GetLuaScript = `
local key = KEYS[1]
local data = redis.call('HGET', key, 'data')
local version = redis.call('HGET', key, 'version')
if not data or not version then
    return nil
end
return {data, version}
`

// RedisSubstrate implements DataSubstrateSPI backed by a Redis/Valkey cluster.
type RedisSubstrate struct {
	client RedisCommander
	prefix string
}

// NewRedisSubstrate creates a distributed shared data substrate instance.
func NewRedisSubstrate(client RedisCommander, keyPrefix string) *RedisSubstrate {
	if keyPrefix == "" {
		keyPrefix = "octpex:sub"
	}
	return &RedisSubstrate{
		client: client,
		prefix: keyPrefix,
	}
}

func (r *RedisSubstrate) formatKey(uri string) string {
	return fmt.Sprintf("%s:%s", r.prefix, uri)
}

func (r *RedisSubstrate) formatLockKey(uri string) string {
	return fmt.Sprintf("%s:lock:%s", r.prefix, uri)
}

// PutArtifact writes an artifact atomically via Lua script enforcing CAS.
func (r *RedisSubstrate) PutArtifact(ctx context.Context, uri string, expectedVersion int64, data []byte, ttl time.Duration) (int64, error) {
	key := r.formatKey(uri)
	ttlMs := ttl.Milliseconds()

	res, err := r.client.Eval(ctx, CASLuaScript, []string{key}, expectedVersion, string(data), ttlMs)
	if err != nil {
		if strings.Contains(err.Error(), "CAS_CONFLICT") || err == ErrCASConflict {
			return 0, fmt.Errorf("%w: %v", ErrCASConflict, err)
		}
		return 0, err
	}

	resStr, ok := res.(string)
	if !ok {
		return 0, fmt.Errorf("unexpected lua response type: %T", res)
	}

	newVersion, err := strconv.ParseInt(resStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse updated version: %w", err)
	}

	return newVersion, nil
}

// GetArtifact fetches artifact data and version atomically.
func (r *RedisSubstrate) GetArtifact(ctx context.Context, uri string) ([]byte, int64, error) {
	key := r.formatKey(uri)
	res, err := r.client.Eval(ctx, GetLuaScript, []string{key})
	if err != nil {
		return nil, 0, err
	}
	if res == nil {
		return nil, 0, ErrNotFound
	}

	arr, ok := res.([]interface{})
	if !ok || len(arr) < 2 {
		return nil, 0, ErrNotFound
	}

	dataStr, ok1 := arr[0].(string)
	versionStr, ok2 := arr[1].(string)
	if !ok1 || !ok2 {
		return nil, 0, fmt.Errorf("invalid artifact payload format in redis")
	}

	version, err := strconv.ParseInt(versionStr, 10, 64)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid version stored in redis: %w", err)
	}

	return []byte(dataStr), version, nil
}

// AcquireLease grabs a distributed lease lock on the given URI using SETNX.
func (r *RedisSubstrate) AcquireLease(ctx context.Context, uri string, workerID string, ttl time.Duration) (string, error) {
	lockKey := r.formatLockKey(uri)
	token := fmt.Sprintf("%s-%d", workerID, time.Now().UnixNano())

	ok, err := r.client.SetNX(ctx, lockKey, token, ttl)
	if err != nil {
		return "", fmt.Errorf("redis lease failed: %w", err)
	}
	if !ok {
		return "", ErrLeaseConflict
	}
	return token, nil
}

const releaseScript = `if redis.call('GET',KEYS[1]) == ARGV[1] then return redis.call('DEL',KEYS[1]) end return 0`

// ReleaseLease explicitly releases the lease token.
func (r *RedisSubstrate) ReleaseLease(ctx context.Context, uri string, leaseToken string) error {
	lockKey := r.formatLockKey(uri)
	_, err := r.client.Eval(ctx, releaseScript, []string{lockKey}, leaseToken)
	return err
}

// DeleteArtifact purges an artifact from Redis.
func (r *RedisSubstrate) DeleteArtifact(ctx context.Context, uri string) error {
	key := r.formatKey(uri)
	_, err := r.client.Del(ctx, key)
	return err
}

// MockRedisCommander provides an in-memory Redis commander simulator for testing CAS Lua scripts.
type MockRedisCommander struct {
	mu     sync.RWMutex
	hashes map[string]map[string]string
	keys   map[string]string
}

// NewMockRedisCommander creates an in-memory Redis commander simulator.
func NewMockRedisCommander() *MockRedisCommander {
	return &MockRedisCommander{
		hashes: make(map[string]map[string]string),
		keys:   make(map[string]string),
	}
}

func (m *MockRedisCommander) Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := keys[0]
	if script == releaseScript {
		if m.keys[keys[0]] == args[0] {
			delete(m.keys, keys[0])
		}
		return int64(0), nil
	}
	if script == CASLuaScript {
		expectedVer := args[0].(int64)
		newData := args[1].(string)

		h, exists := m.hashes[key]
		if !exists {
			if expectedVer != 0 {
				return nil, fmt.Errorf("CAS_CONFLICT: artifact does not exist")
			}
			m.hashes[key] = map[string]string{
				"data":    newData,
				"version": "1",
			}
			return "1", nil
		}

		currVer, _ := strconv.ParseInt(h["version"], 10, 64)
		if expectedVer == 0 {
			return nil, fmt.Errorf("CAS_CONFLICT: artifact already exists")
		}
		if currVer != expectedVer {
			return nil, fmt.Errorf("CAS_CONFLICT: version mismatch")
		}

		nextVer := currVer + 1
		h["data"] = newData
		h["version"] = strconv.FormatInt(nextVer, 10)
		return strconv.FormatInt(nextVer, 10), nil
	}

	if script == GetLuaScript {
		h, exists := m.hashes[key]
		if !exists {
			return nil, nil
		}
		return []interface{}{h["data"], h["version"]}, nil
	}

	return nil, fmt.Errorf("unsupported mock script")
}

func (m *MockRedisCommander) Get(ctx context.Context, key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, ok := m.keys[key]
	if !ok {
		return "", ErrNotFound
	}
	return val, nil
}

func (m *MockRedisCommander) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.keys[key]; ok {
		return false, nil
	}
	m.keys[key] = fmt.Sprintf("%v", value)
	return true, nil
}

func (m *MockRedisCommander) Del(ctx context.Context, keys ...string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var count int64
	for _, k := range keys {
		if _, ok := m.hashes[k]; ok {
			delete(m.hashes, k)
			count++
		}
		if _, ok := m.keys[k]; ok {
			delete(m.keys, k)
			count++
		}
	}
	return count, nil
}
