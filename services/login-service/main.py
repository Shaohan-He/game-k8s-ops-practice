import hashlib
import logging
import secrets
from datetime import UTC, datetime

from fastapi import HTTPException, Request
from pydantic import BaseModel, Field

from common.app import create_app
from common.metrics import BUSINESS_EVENTS

app = create_app("Login Service")
logger = logging.getLogger("login-service")


class LoginRequest(BaseModel):
    username: str = Field(min_length=3, max_length=64)
    password: str = Field(min_length=6, max_length=128)


class LogoutRequest(BaseModel):
    token: str = Field(min_length=16)


@app.post("/login", tags=["login"])
async def login(payload: LoginRequest, request: Request):
    infra = request.app.state.infrastructure
    password_hash = hashlib.sha256(payload.password.encode()).hexdigest()
    async with infra.mysql.acquire() as conn:
        async with conn.cursor() as cursor:
            await cursor.execute(
                "SELECT id, username FROM users WHERE username=%s AND password_hash=%s",
                (payload.username, password_hash),
            )
            user = await cursor.fetchone()
            if not user:
                BUSINESS_EVENTS.labels("login-service", "login", "failed").inc()
                await cursor.execute(
                    "INSERT INTO login_records(user_id, username, action, success, ip_address) "
                    "VALUES(NULL, %s, 'login', 0, %s)",
                    (payload.username, request.client.host if request.client else None),
                )
                raise HTTPException(status_code=401, detail="用户名或密码错误")

            player_id, username = user
            token = secrets.token_urlsafe(32)
            session = {"player_id": str(player_id), "username": username}
            await infra.redis.hset(f"session:{token}", mapping=session)
            await infra.redis.expire(f"session:{token}", 3600)
            await infra.redis.set(f"player:online:{player_id}", "1", ex=3600)
            await cursor.execute(
                "INSERT INTO login_records(user_id, username, action, success, ip_address) "
                "VALUES(%s, %s, 'login', 1, %s)",
                (player_id, username, request.client.host if request.client else None),
            )

    await infra.publish(
        "game.login.events",
        "player_login",
        {"player_id": player_id, "username": username},
    )
    BUSINESS_EVENTS.labels("login-service", "login", "success").inc()
    return {
        "token": token,
        "player_id": player_id,
        "username": username,
        "expires_in": 3600,
    }


@app.post("/logout", tags=["login"])
async def logout(payload: LogoutRequest, request: Request):
    infra = request.app.state.infrastructure
    session = await infra.redis.hgetall(f"session:{payload.token}")
    if not session:
        raise HTTPException(status_code=404, detail="会话不存在或已过期")
    player_id = session["player_id"]
    await infra.redis.delete(f"session:{payload.token}", f"player:online:{player_id}")
    async with infra.mysql.acquire() as conn:
        async with conn.cursor() as cursor:
            await cursor.execute(
                "INSERT INTO login_records(user_id, username, action, success, logout_at) "
                "VALUES(%s, %s, 'logout', 1, %s)",
                (player_id, session["username"], datetime.now(UTC).replace(tzinfo=None)),
            )
    await infra.publish(
        "game.login.events", "player_logout", {"player_id": player_id}
    )
    BUSINESS_EVENTS.labels("login-service", "logout", "success").inc()
    return {"message": "logout success", "player_id": int(player_id)}

