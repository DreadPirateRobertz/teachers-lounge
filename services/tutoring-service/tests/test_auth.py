"""Tests for JWT auth validation."""
import time
from datetime import datetime, timezone
from unittest.mock import MagicMock
from uuid import UUID, uuid4

import pytest
from fastapi import HTTPException
from fastapi.security import HTTPAuthorizationCredentials
from jose import jwt

# Override settings before import to avoid needing real DB/gateway in tests
import os
os.environ.setdefault("DATABASE_URL", "postgresql+asyncpg://test:test@localhost:5432/test")
os.environ.setdefault("AI_GATEWAY_KEY", "test-key")
os.environ.setdefault("JWT_SECRET", "test-secret-that-is-long-enough-for-hs256")

from app.auth import JWTClaims, require_auth
from app.config import settings


def _make_token(sub: str, account_type: str = "standard", exp_offset: int = 900) -> str:
    payload = {
        "sub": sub,
        "account_type": account_type,
        "iat": int(time.time()),
        "exp": int(time.time()) + exp_offset,
    }
    return jwt.encode(payload, settings.jwt_secret, algorithm=settings.jwt_algorithm)


def _creds(token: str) -> HTTPAuthorizationCredentials:
    return HTTPAuthorizationCredentials(scheme="Bearer", credentials=token)


def test_valid_token_returns_claims():
    user_id = uuid4()
    token = _make_token(str(user_id), account_type="standard")
    claims = require_auth(_creds(token))
    assert claims.user_id == user_id
    assert claims.account_type == "standard"


def test_minor_account_type():
    user_id = uuid4()
    token = _make_token(str(user_id), account_type="minor")
    claims = require_auth(_creds(token))
    assert claims.account_type == "minor"


def test_expired_token_raises_401():
    user_id = uuid4()
    token = _make_token(str(user_id), exp_offset=-1)
    with pytest.raises(HTTPException) as exc_info:
        require_auth(_creds(token))
    assert exc_info.value.status_code == 401
    assert "expired" in exc_info.value.detail.lower()


def test_invalid_token_raises_401():
    with pytest.raises(HTTPException) as exc_info:
        require_auth(_creds("not.a.valid.jwt"))
    assert exc_info.value.status_code == 401


def test_wrong_secret_raises_401():
    user_id = uuid4()
    token = jwt.encode(
        {"sub": str(user_id), "exp": int(time.time()) + 900},
        "wrong-secret",
        algorithm="HS256",
    )
    with pytest.raises(HTTPException) as exc_info:
        require_auth(_creds(token))
    assert exc_info.value.status_code == 401


def test_missing_sub_raises_401():
    payload = {"account_type": "standard", "exp": int(time.time()) + 900}
    token = jwt.encode(payload, settings.jwt_secret, algorithm=settings.jwt_algorithm)
    with pytest.raises(HTTPException) as exc_info:
        require_auth(_creds(token))
    assert exc_info.value.status_code == 401
