"""JWT authentication helpers for the analytics service.

All analytics endpoints require a valid JWT issued by the user-service.
The token is passed as a Bearer credential in the Authorization header.
"""
from fastapi import HTTPException, Security
from fastapi.security import HTTPAuthorizationCredentials, HTTPBearer
from jose import JWTError, jwt

from .config import settings

bearer_scheme = HTTPBearer(auto_error=False)


def require_auth(
    credentials: HTTPAuthorizationCredentials | None = Security(bearer_scheme),
) -> str:
    """Validate a Bearer JWT and return the subject claim.

    Intended for use as a FastAPI dependency via ``Depends(require_auth)``.

    Args:
        credentials: HTTP Authorization header parsed by FastAPI's HTTPBearer
            scheme.  ``None`` when the header is absent.

    Returns:
        The ``sub`` claim from the validated token payload (the user's UUID).

    Raises:
        HTTPException: 401 Unauthorized when the header is missing, the token
            is malformed, the signature is invalid, or the ``sub`` claim is
            absent.
    """
    if not credentials:
        raise HTTPException(status_code=401, detail="Missing token")
    try:
        payload = jwt.decode(
            credentials.credentials,
            settings.jwt_secret,
            algorithms=[settings.jwt_algorithm],
            audience=settings.jwt_audience,
        )
        user_id: str | None = payload.get("sub")
        if not user_id:
            raise HTTPException(status_code=401, detail="Invalid token")
        return user_id
    except JWTError:
        raise HTTPException(status_code=401, detail="Invalid token")
