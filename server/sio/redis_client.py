"""
Redis 클라이언트 싱글톤

선택 이유:
- redis-py async: 공식 클라이언트, asyncio 네이티브
- Sorted Set: score를 만료 시각으로 사용하여 TTL 관리
  → ZADD로 추가, ZCOUNT로 활성 카운트, ZREMRANGEBYSCORE로 만료 정리
  → Redis의 key-level TTL 대신 Sorted Set을 쓰는 이유:
    한 격자에 여러 이벤트가 각각 독립 TTL을 가져야 하기 때문
"""

import redis.asyncio as aioredis

REDIS_URL = "redis://localhost:6379"


class RedisClient:
    def __init__(self):
        self.redis: aioredis.Redis | None = None

    async def initialize(self):
        self.redis = aioredis.from_url(REDIS_URL, decode_responses=True)
        await self.redis.ping()
        print("[Redis] Connected")

    async def close(self):
        if self.redis:
            await self.redis.close()

    def get(self) -> aioredis.Redis:
        if not self.redis:
            raise RuntimeError("Redis not initialized")
        return self.redis


redis_client = RedisClient()
