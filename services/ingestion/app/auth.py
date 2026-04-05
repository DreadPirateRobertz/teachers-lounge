"""JWT authentication for the ingestion service.

Validates HS256 tokens issued by the User Service. Tokens carry:
  aud        — audience: "teacherslounge-services"  (validated here)
  uid        — user UUID string  (preferred)
  sub        — user UUID string  (fallback for forward-compatibility)
  email      — user email address
  exp        — expiry (validated by python-jose)

Usage::

    @router.post("/upload")
    async def upload(user_id: UUID = Depends(require_auth)):
        ...
"""
from uuid import UUID

from fastapi import Depends, HTTPException, status
from fastapi.security import HTTPAuthorizationCredentials, HTTPBearer
from jose import ExpiredSignatureError, JWTError, jwt

from .config import settings

_bearer = HTTPBearer(auto_error=True)


def require_auth(
    credentials: HTTPAuthorizationCredentials = Depends(_bearer),
) -> UUID:
    """Validate a Bearer JWT and return the caller's user UUID.

    Validates signature, expiry, and audience. Extracts the ``uid`` claim
    (preferred) or falls back to ``sub`` for forward-compatibility while
    older User Service tokens may omit ``uid``.

    Args:
        credentials: HTTP Authorization header parsed by FastAPI's HTTPBearer
            scheme.

    Returns:
        The caller's user UUID extracted from the validated token.

    Raises:
        HTTPException: 401 when the token is missing, expired, has wrong
            audience, invalid signature, or lacks a user-id claim.
    """
    token = credentials.credentials
    try:
        payload = jwt.decode(
            token,
            settings.jwt_secret,
            algorithms=[settings.jwt_algorithm],
            audience=settings.jwt_audience,
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

    # python-jose does not raise when `aud` is absent; enforce it explicitly.
    if not payload.get("aud"):
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid token",
            headers={"WWW-Authenticate": "Bearer"},
        )

    uid = payload.get("uid") or payload.get("sub")
    if not uid:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Missing uid claim",
            headers={"WWW-Authenticate": "Bearer"},
        )

    try:
        return UUID(uid)
    except ValueError:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid uid claim",
            headers={"WWW-Authenticate": "Bearer"},
        )
