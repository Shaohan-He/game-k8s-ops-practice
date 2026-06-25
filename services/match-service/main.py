import time

from fastapi import HTTPException, Request
from pydantic import BaseModel, Field

from common.app import create_app
from common.metrics import BUSINESS_EVENTS

app = create_app("Match Service")
QUEUE_KEY = "match:queue"


class MatchRequest(BaseModel):
    player_id: int = Field(gt=0)
    mode: str = Field(default="ranked", pattern="^(ranked|casual)$")


@app.post("/match", tags=["match"])
async def join_match(payload: MatchRequest, request: Request):
    infra = request.app.state.infrastructure
    if not await infra.redis.exists(f"player:online:{payload.player_id}"):
        raise HTTPException(status_code=409, detail="玩家不在线，无法加入匹配")
    queue_member = f"{payload.mode}:{payload.player_id}"
    await infra.redis.zadd(QUEUE_KEY, {queue_member: time.time()})
    await infra.publish(
        "game.match.events",
        "match_joined",
        {"player_id": payload.player_id, "mode": payload.mode},
    )
    BUSINESS_EVENTS.labels("match-service", "match_joined", "success").inc()
    return {"status": "queued", "player_id": payload.player_id, "mode": payload.mode}


@app.get("/match/status", tags=["match"])
async def match_status(request: Request, player_id: int, mode: str = "ranked"):
    infra = request.app.state.infrastructure
    member = f"{mode}:{player_id}"
    rank = await infra.redis.zrank(QUEUE_KEY, member)
    if rank is None:
        return {"status": "not_queued", "player_id": player_id}
    score = await infra.redis.zscore(QUEUE_KEY, member)
    return {
        "status": "queued",
        "player_id": player_id,
        "mode": mode,
        "queue_position": rank + 1,
        "waiting_seconds": max(0, int(time.time() - score)),
    }

