"""valtag auth gate for the LiteLLM proxy — ask-first, fail-closed.

Every request must present a valtag derived key
(`Authorization: Bearer valtag_dk_…`). The key is validated against
valtag's gateway authorize endpoint per request:

- 200 → the caller holds an approved lease; the request proceeds
- 401  → invalid/expired/revoked credential → denied
- 403  → policy denied
- valtag unreachable → 503 fail-closed (never fail open)

Wire it in the LiteLLM config:

    general_settings:
      custom_auth: valtag_gate.user_api_key_auth

Env: VALTAG_AUTHORIZE_URL (default http://127.0.0.1:10256/gateway/v1/authorize),
VALTAG_TIMEOUT_SECONDS (default 3).
"""

import logging
import os

import httpx
from fastapi import HTTPException, Request

from litellm.proxy._types import UserAPIKeyAuth

log = logging.getLogger("valtag_gate")

AUTHORIZE_URL = os.getenv(
    "VALTAG_AUTHORIZE_URL", "http://127.0.0.1:10256/gateway/v1/authorize"
)
TIMEOUT = float(os.getenv("VALTAG_TIMEOUT_SECONDS", "3"))


async def user_api_key_auth(request: Request, api_key: str) -> UserAPIKeyAuth:
    # api_key = the presented bearer value (valtag_dk_…). Forward it to
    # valtag's authorize endpoint, which validates the lease and returns the
    # real upstream credential as headers — LiteLLM never holds it.
    key = api_key
    if key.startswith("Bearer "):
        key = key[7:]

    try:
        async with httpx.AsyncClient(timeout=TIMEOUT) as client:
            resp = await client.post(
                AUTHORIZE_URL,
                headers={"Authorization": f"Bearer {key}"},
            )
    except Exception as e:
        # fail-closed: valtag unreachable → deny
        log.exception("valtag_gate: authorize call failed")
        raise HTTPException(
            status_code=503,
            detail=f"valtag unavailable — fail-closed ({type(e).__name__})",
        )

    if resp.status_code != 200:
        # 401 invalid/expired/revoked · 403 policy denied — pass the status
        # through so the caller sees the gate's verdict.
        raise HTTPException(
            status_code=resp.status_code,
            detail="valtag denied: invalid or expired credential",
        )

    agent_id = resp.headers.get("x-valtag-agent-id") or "valtag-agent"
    return UserAPIKeyAuth(api_key=api_key, user_id=agent_id)
