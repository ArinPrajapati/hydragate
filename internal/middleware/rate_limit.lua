local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])

local bucket = redis.call("HMGET", key, "tokens", "last_refill")
local tokens = tonumber(bucket[1])
local last_refill = tonumber(bucket[2])

if not tokens then
    tokens = capacity
    last_refill = now
end

local time_passed = math.max(0, now - last_refill)
local new_tokens = math.floor(time_passed * (refill_rate / 60))

if new_tokens > 0 then
    tokens = math.min(capacity, tokens + new_tokens)
    last_refill = now
end

if tokens >= requested then
    tokens = tokens - requested
    redis.call("HMSET", key, "tokens", tokens, "last_refill", last_refill)
    redis.call("EXPIRE", key, 300) -- expire after 5 mins inactivity
    return 1 -- allowed
else
	redis.call("HMSET", key, "tokens", tokens, "last_refill", last_refill)
    redis.call("EXPIRE", key, 300)
    return 0 -- rejected
end
