"""
JWT validation middleware — validates access tokens issued by the User Service.

The User Service (tl-38s) signs JWTs with a shared HS256 secret.

Expected claims:
  sub          — user UUID
  account_type — "standard" | "minor"
  exp          — expiry (15 min TTL from User Service)
  iat          — issued-at

Usage:
    @router.post("/sessions")
    async def create(body: ..., user: JWTClaims = Depends(require_auth)):
        # user.user_id is the validated UUID
"""
from uuid import UUID

from fastapi import Depends, HTTPException, status
from fastapi.security import HTTPAuthorizationCredentials, HTTPBearer
from jose import ExpiredSignatureError, JWTError, jwt
from pydantic import BaseModel

from .config import settings

_bearer = HTTPBearer(auto_error=True)


class JWTClaims(BaseModel):
    user_id: UUID
    account_type: str  # "standard" | "minor"


def require_auth(
    credentials: HTTPAuthorizationCredentials = Depends(_bearer),
) -> JWTClaims:
    """FastAPI dependency — validates Bearer JWT and returns parsed claims."""
    token = credentials.credentials
    try:
        payload = jwt.decode(
            token,
            settings.jwt_secret,
            algorithms=[settings.jwt_algorithm],
        )
    except ExpiredSignatureError:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Token expired",
            headers={"WWW-Authenticate": "Bearer"},
        )
    except JWTError:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid token",
            headers={"WWW-Authenticate": "Bearer"},
        )

    sub = payload.get("sub")
    if sub is None:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="Missing sub claim")

    return JWTClaims(
        user_id=UUID(sub),
        account_type=payload.get("account_type", "standard"),
    )
