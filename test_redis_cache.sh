#!/bin/bash

echo "=== Redis Cache Test Script ==="
echo ""

# Test 1: Check if Redis is running
echo "1. Testing Redis connection..."
if redis-cli ping > /dev/null 2>&1; then
    echo "✅ Redis is running"
else
    echo "❌ Redis is NOT running"
    exit 1
fi
echo ""

# Test 2: Clear old cache entries
echo "2. Clearing old cache entries..."
redis-cli del $(redis-cli keys "gateway:cache:*" 2>/dev/null) 2>/dev/null
echo "✅ Old cache cleared"
echo ""

# Test 3: Make a request to /api/date
echo "3. Making request to /api/date..."
curl -s http://localhost:8080/api/date > /dev/null
echo "✅ Request made"
echo ""

# Test 4: Check if key exists in Redis
echo "4. Checking if cache key exists..."
KEY="gateway:cache:GET:api:date"
if redis-cli exists "$KEY" > /dev/null 2>&1; then
    echo "✅ Cache key exists: $KEY"
    TTL=$(redis-cli ttl "$KEY")
    echo "   TTL: $TTL seconds"
    echo ""
    echo "5. Cache content preview:"
    redis-cli get "$KEY" | head -c 200
    echo "..."
else
    echo "❌ Cache key NOT found: $KEY"
    echo ""
    echo "Listing all gateway:cache keys:"
    redis-cli keys "gateway:cache:*"
fi
echo ""

# Test 6: Make second request (should hit cache)
echo "6. Making second request (should be cache hit)..."
curl -s http://localhost:8080/api/date > /dev/null
echo "✅ Second request made"
echo ""

# Test 7: Check logs for cache hit
echo "7. Check server logs for 'cache hit' message"
echo "   (Look in your server terminal/output)"
echo ""

echo "=== Test Complete ==="
