import secrets
from datetime import UTC, datetime

from fastapi import HTTPException, Request
from pydantic import BaseModel, Field

from common.app import create_app
from common.metrics import BUSINESS_EVENTS

app = create_app("Room Service")


class RoomCreateRequest(BaseModel):
    owner_id: int = Field(gt=0)
    max_players: int = Field(default=4, ge=2, le=10)


class RoomActionRequest(BaseModel):
    room_id: str = Field(min_length=6, max_length=32)
    player_id: int = Field(gt=0)


@app.post("/room/create", tags=["room"])
async def create_room(payload: RoomCreateRequest, request: Request):
    infra = request.app.state.infrastructure
    if not await infra.redis.exists(f"player:online:{payload.owner_id}"):
        raise HTTPException(status_code=409, detail="房主不在线")
    room_id = secrets.token_hex(4)
    room_key = f"room:{room_id}"
    room = {
        "room_id": room_id,
        "owner_id": str(payload.owner_id),
        "max_players": str(payload.max_players),
        "status": "waiting",
        "created_at": datetime.now(UTC).isoformat(),
    }
    async with infra.redis.pipeline(transaction=True) as pipe:
        pipe.hset(room_key, mapping=room)
        pipe.sadd(f"{room_key}:players", payload.owner_id)
        pipe.expire(room_key, 7200)
        pipe.expire(f"{room_key}:players", 7200)
        await pipe.execute()
    async with infra.mysql.acquire() as conn:
        async with conn.cursor() as cursor:
            await cursor.execute(
                "INSERT INTO rooms(room_id, owner_id, max_players, status) VALUES(%s,%s,%s,'waiting')",
                (room_id, payload.owner_id, payload.max_players),
            )
            await cursor.execute(
                "INSERT INTO room_players(room_id, player_id, joined_at) VALUES(%s,%s,NOW())",
                (room_id, payload.owner_id),
            )
    await infra.publish(
        "game.room.events",
        "room_created",
        {"room_id": room_id, "owner_id": payload.owner_id},
    )
    BUSINESS_EVENTS.labels("room-service", "room_created", "success").inc()
    return room


@app.post("/room/join", tags=["room"])
async def join_room(payload: RoomActionRequest, request: Request):
    infra = request.app.state.infrastructure
    room_key = f"room:{payload.room_id}"
    room = await infra.redis.hgetall(room_key)
    if not room:
        raise HTTPException(status_code=404, detail="房间不存在或已过期")
    players_key = f"{room_key}:players"
    if await infra.redis.scard(players_key) >= int(room["max_players"]):
        raise HTTPException(status_code=409, detail="房间已满")
    await infra.redis.sadd(players_key, payload.player_id)
    async with infra.mysql.acquire() as conn:
        async with conn.cursor() as cursor:
            await cursor.execute(
                "INSERT INTO room_players(room_id, player_id, joined_at) VALUES(%s,%s,NOW()) "
                "ON DUPLICATE KEY UPDATE joined_at=NOW(), left_at=NULL",
                (payload.room_id, payload.player_id),
            )
    await infra.publish("game.room.events", "room_joined", payload.model_dump())
    BUSINESS_EVENTS.labels("room-service", "room_joined", "success").inc()
    return {"message": "join success", **payload.model_dump()}


@app.post("/room/leave", tags=["room"])
async def leave_room(payload: RoomActionRequest, request: Request):
    infra = request.app.state.infrastructure
    room_key = f"room:{payload.room_id}"
    if not await infra.redis.exists(room_key):
        raise HTTPException(status_code=404, detail="房间不存在或已过期")
    await infra.redis.srem(f"{room_key}:players", payload.player_id)
    async with infra.mysql.acquire() as conn:
        async with conn.cursor() as cursor:
            await cursor.execute(
                "UPDATE room_players SET left_at=NOW() WHERE room_id=%s AND player_id=%s",
                (payload.room_id, payload.player_id),
            )
    await infra.publish("game.room.events", "room_left", payload.model_dump())
    BUSINESS_EVENTS.labels("room-service", "room_left", "success").inc()
    return {"message": "leave success", **payload.model_dump()}


@app.get("/room/status", tags=["room"])
async def room_status(room_id: str, request: Request):
    infra = request.app.state.infrastructure
    room_key = f"room:{room_id}"
    room = await infra.redis.hgetall(room_key)
    if not room:
        raise HTTPException(status_code=404, detail="房间不存在或已过期")
    players = sorted(
        int(item) for item in await infra.redis.smembers(f"{room_key}:players")
    )
    return {**room, "players": players, "player_count": len(players)}

