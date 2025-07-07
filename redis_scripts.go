package lock

// Redis Lua脚本优化实现
// 通过原子性脚本减少网络往返次数，提升性能

const (
	// ===== 简单锁优化脚本 =====
	
	// SimpleLockAcquireScript 原子性获取简单锁
	SimpleLockAcquireScript = `
		-- KEYS[1]: lock key
		-- ARGV[1]: lock value (holder identifier)  
		-- ARGV[2]: TTL in seconds
		if redis.call("SET", KEYS[1], ARGV[1], "NX", "EX", ARGV[2]) then
			return 1
		end
		return 0
	`
	
	// SimpleLockReleaseScript 原子性释放简单锁
	SimpleLockReleaseScript = `
		-- KEYS[1]: lock key
		-- ARGV[1]: lock value (holder identifier)
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			redis.call("DEL", KEYS[1])
			return 1
		end
		return 0
	`
	
	// SimpleLockExtendScript 原子性续期简单锁
	SimpleLockExtendScript = `
		-- KEYS[1]: lock key
		-- ARGV[1]: lock value (holder identifier)
		-- ARGV[2]: TTL in seconds
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			redis.call("EXPIRE", KEYS[1], ARGV[2])
			return 1
		end
		return 0
	`
	
	// SimpleLockStatusScript 原子性获取锁状态和TTL
	SimpleLockStatusScript = `
		-- KEYS[1]: lock key
		-- Returns: {exists, ttl_seconds}
		local exists = redis.call("EXISTS", KEYS[1])
		if exists == 1 then
			local ttl = redis.call("TTL", KEYS[1])
			return {1, ttl}
		else
			return {0, -1}
		end
	`

	// ===== 读写锁优化脚本 =====
	
	// RWLockAcquireReadScript 原子性获取读锁
	RWLockAcquireReadScript = `
		-- KEYS[1]: read lock key
		-- KEYS[2]: write lock key  
		-- ARGV[1]: TTL in seconds
		if redis.call("EXISTS", KEYS[2]) == 0 then
			redis.call("INCR", KEYS[1])
			redis.call("EXPIRE", KEYS[1], ARGV[1])
			return 1
		end
		return 0
	`
	
	// RWLockReleaseReadScript 原子性释放读锁
	RWLockReleaseReadScript = `
		-- KEYS[1]: read lock key
		local count = redis.call("GET", KEYS[1])
		if not count then
			return 0
		end
		count = tonumber(count)
		if count > 1 then
			redis.call("DECR", KEYS[1])
			return 1
		else
			redis.call("DEL", KEYS[1])
			return 1
		end
	`
	
	// RWLockAcquireWriteScript 原子性获取写锁
	RWLockAcquireWriteScript = `
		-- KEYS[1]: read lock key
		-- KEYS[2]: write lock key
		-- ARGV[1]: lock value (holder identifier)
		-- ARGV[2]: TTL in seconds
		if redis.call("EXISTS", KEYS[1]) == 0 and redis.call("EXISTS", KEYS[2]) == 0 then
			redis.call("SET", KEYS[2], ARGV[1], "EX", ARGV[2])
			return 1
		end
		return 0
	`
	
	// RWLockReleaseWriteScript 原子性释放写锁
	RWLockReleaseWriteScript = `
		-- KEYS[1]: write lock key
		-- ARGV[1]: lock value (holder identifier)
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			redis.call("DEL", KEYS[1])
			return 1
		end
		return 0
	`
	
	// RWLockStatusScript 原子性获取读写锁状态
	RWLockStatusScript = `
		-- KEYS[1]: read lock key
		-- KEYS[2]: write lock key
		-- Returns: {read_count, write_locked}
		local read_count = redis.call("GET", KEYS[1])
		if not read_count then
			read_count = 0
		else
			read_count = tonumber(read_count)
		end
		
		local write_locked = redis.call("EXISTS", KEYS[2])
		return {read_count, write_locked}
	`

	// ===== 公平锁优化脚本 =====
	
	// FairLockTryAcquireScript 原子性尝试获取公平锁
	FairLockTryAcquireScript = `
		-- KEYS[1]: lock key
		-- KEYS[2]: queue key
		-- ARGV[1]: lock value (holder identifier)
		-- ARGV[2]: lock TTL in seconds
		-- ARGV[3]: queue TTL in seconds
		-- Returns: 1=success, 0=failed, -1=queued
		
		-- 1. 检查锁是否存在
		local lock_exists = redis.call("EXISTS", KEYS[1])
		if lock_exists == 0 then
			-- 锁不存在，直接获取
			redis.call("SET", KEYS[1], ARGV[1], "EX", ARGV[2])
			return 1
		end
		
		-- 2. 锁已存在，加入队列
		redis.call("RPUSH", KEYS[2], ARGV[1])
		redis.call("EXPIRE", KEYS[2], ARGV[3])
		
		-- 3. 检查是否在队首
		local first = redis.call("LINDEX", KEYS[2], 0)
		if first == ARGV[1] then
			-- 在队首，尝试获取锁
			local current_holder = redis.call("GET", KEYS[1])
			if not current_holder then
				-- 锁已释放，获取成功
				redis.call("SET", KEYS[1], ARGV[1], "EX", ARGV[2])
				redis.call("LPOP", KEYS[2])
				return 1
			end
		end
		
		-- 4. 不在队首或锁仍被持有
		return -1
	`
	
	// FairLockAcquireScript 原子性获取公平锁（阻塞版本的核心逻辑）
	FairLockAcquireScript = `
		-- KEYS[1]: lock key
		-- KEYS[2]: queue key
		-- ARGV[1]: lock value (holder identifier)
		-- ARGV[2]: lock TTL in seconds
		-- Returns: 1=success, 0=failed (not first in queue), -1=lock held by others
		
		-- 1. 检查队列中的位置
		local first = redis.call("LINDEX", KEYS[2], 0)
		if first ~= ARGV[1] then
			-- 不在队首，无法获取锁
			return 0
		end
		
		-- 2. 在队首，尝试获取锁
		local current_holder = redis.call("GET", KEYS[1])
		if not current_holder then
			-- 锁已释放，获取成功
			redis.call("SET", KEYS[1], ARGV[1], "EX", ARGV[2])
			redis.call("LPOP", KEYS[2])
			return 1
		end
		
		-- 3. 锁仍被持有
		return -1
	`
	
	// FairLockReleaseScript 原子性释放公平锁
	FairLockReleaseScript = `
		-- KEYS[1]: lock key
		-- KEYS[2]: queue key
		-- ARGV[1]: lock value (holder identifier)
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			redis.call("DEL", KEYS[1])
			-- 清理队列中的自己（防止重复）
			redis.call("LREM", KEYS[2], 0, ARGV[1])
			return 1
		end
		return 0
	`
	
	// FairLockExtendScript 原子性续期公平锁
	FairLockExtendScript = `
		-- KEYS[1]: lock key
		-- ARGV[1]: lock value (holder identifier)
		-- ARGV[2]: TTL in milliseconds
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("PEXPIRE", KEYS[1], ARGV[2])
		else
			return 0
		end
	`
	
	// FairLockStatusScript 原子性获取公平锁状态
	FairLockStatusScript = `
		-- KEYS[1]: lock key
		-- KEYS[2]: queue key
		-- ARGV[1]: lock value (holder identifier) - optional
		-- Returns: {locked, ttl_ms, queue_length, position}
		
		local locked = redis.call("EXISTS", KEYS[1])
		local ttl = -1
		if locked == 1 then
			ttl = redis.call("PTTL", KEYS[1])
		end
		
		local queue_length = redis.call("LLEN", KEYS[2])
		local position = -1
		
		-- 如果提供了value，查找在队列中的位置
		if ARGV[1] then
			local queue = redis.call("LRANGE", KEYS[2], 0, -1)
			for i, v in ipairs(queue) do
				if v == ARGV[1] then
					position = i - 1  -- 0-based position
					break
				end
			end
		end
		
		return {locked, ttl, queue_length, position}
	`
	
	// FairLockCleanupScript 清理公平锁队列中的特定值
	FairLockCleanupScript = `
		-- KEYS[1]: queue key
		-- ARGV[1]: value to remove
		return redis.call("LREM", KEYS[1], 0, ARGV[1])
	`
)
