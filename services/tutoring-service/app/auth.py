"""
JWT validation — validates access tokens issued by the User Service (tl-38s).

User Service signs HS256 JWTs with the following custom claims:
  uid        — user UUID string  (also mirrored in RegisteredClaims.sub)
  email      — user email address
  acct       — account type: "standard" | "minor"
  sub_status — subscription status (optional): "active" | "trial" | "past_due" etc.

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
    email: str
    account_type: str   # "standard" | "minor"
    sub_status: str     # "active" | "trial" | "past_due" | "cancelled" | ""


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
            # verify_aud=False: User Service does not set an "aud" claim yet.
            # Tracked in tl-eam — enable once User Service adds aud: "teacherslounge-services".
            options={"verify_aud": False},
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

    # User Service sets "uid" custom claim AND RegisteredClaims.sub to the user UUID.
    uid = payload.get("uid") or payload.get("sub")
    if not uid:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Missing uid claim",
        )

    return JWTClaims(
        user_id=UUID(uid),
        email=payload.get("email", ""),
        account_type=payload.get("acct", "standard"),
        sub_status=payload.get("sub_status", ""),
    )
