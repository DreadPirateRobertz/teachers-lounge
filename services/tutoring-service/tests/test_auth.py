"""
Unit tests for JWT validation in auth.py.

All tests use a self-signed HS256 token — no real User Service needed.
The User Service signs tokens with custom claims:
  aud        — audience: "teacherslounge-services"
  uid        — user UUID
  acct       — account type ("standard" | "minor")
  sub_status — subscription status (optional)
"""
import time
import uuid

import pytest
from fastapi import HTTPException
from jose import jwt

from app.auth import JWTClaims, require_auth
from app.config import settings

SECRET = "test-jwt-secret"
ALGORITHM = "HS256"
AUDIENCE = "teacherslounge-services"
USER_ID = str(uuid.uuid4())


def _make_token(
    uid=USER_ID,
    email="nova@test.com",
    acct="standard",
    sub_status="active",
    exp_offset=3600,
    secret=SECRET,
    audience=AUDIENCE,
    extra: dict | None = None,
) -> str:
    payload = {
        "aud": audience,
        "uid": uid,
        "email": email,
        "acct": acct,
        "sub_status": sub_status,
        "exp": int(time.time()) + exp_offset,
    }
    if extra:
        payload.update(extra)
    return jwt.encode(payload, secret, algorithm=ALGORITHM)


def _make_credentials(token: str):
    """Minimal stand-in for HTTPAuthorizationCredentials."""
    class _Creds:
        credentials = token
    return _Creds()


@pytest.fixture(autouse=True)
def _patch_jwt_secret(patch_settings):
    patch_settings(jwt_secret=SECRET, jwt_algorithm=ALGORITHM, jwt_audience=AUDIENCE)


# ── happy path ────────────────────────────────────────────────────────────────

def test_valid_token_returns_claims():
    token = _make_token()
    claims = require_auth(_make_credentials(token))
    assert isinstance(claims, JWTClaims)
    assert str(claims.user_id) == USER_ID
    assert claims.email == "nova@test.com"
    assert claims.account_type == "standard"
    assert claims.sub_status == "active"


def test_minor_account_type():
    token = _make_token(acct="minor")
    claims = require_auth(_make_credentials(token))
    assert claims.account_type == "minor"


def test_sub_status_preserved():
    token = _make_token(sub_status="trial")
    claims = require_auth(_make_credentials(token))
    assert claims.sub_status == "trial"


def test_uid_takes_precedence_over_sub():
    other_id = str(uuid.uuid4())
    token = _make_token(uid=USER_ID, extra={"sub": other_id})
    claims = require_auth(_make_credentials(token))
    assert str(claims.user_id) == USER_ID   # uid wins


def test_sub_used_as_fallback_when_uid_missing():
    payload = {
        "aud": AUDIENCE,
        "sub": USER_ID,
        "email": "fallback@test.com",
        "acct": "standard",
        "sub_status": "",
        "exp": int(time.time()) + 3600,
    }
    token = jwt.encode(payload, SECRET, algorithm=ALGORITHM)
    claims = require_auth(_make_credentials(token))
    assert str(claims.user_id) == USER_ID


# ── error cases ───────────────────────────────────────────────────────────────

def test_expired_token_raises_401():
    token = _make_token(exp_offset=-1)
    with pytest.raises(HTTPException) as exc_info:
        require_auth(_make_credentials(token))
    assert exc_info.value.status_code == 401
    assert "expired" in exc_info.value.detail.lower()


def test_wrong_secret_raises_401():
    token = _make_token(secret="wrong-secret")
    with pytest.raises(HTTPException) as exc_info:
        require_auth(_make_credentials(token))
    assert exc_info.value.status_code == 401


def test_missing_uid_and_sub_raises_401():
    payload = {
        "aud": AUDIENCE,
        "email": "ghost@test.com",
        "acct": "standard",
        "exp": int(time.time()) + 3600,
    }
    token = jwt.encode(payload, SECRET, algorithm=ALGORITHM)
    with pytest.raises(HTTPException) as exc_info:
        require_auth(_make_credentials(token))
    assert exc_info.value.status_code == 401
    assert "uid" in exc_info.value.detail.lower()


def test_wrong_audience_raises_401():
    token = _make_token(audience="wrong-service")
    with pytest.raises(HTTPException) as exc_info:
        require_auth(_make_credentials(token))
    assert exc_info.value.status_code == 401


def test_missing_audience_raises_401():
    payload = {
        "uid": USER_ID,
        "email": "nova@test.com",
        "acct": "standard",
        "sub_status": "active",
        "exp": int(time.time()) + 3600,
    }
    token = jwt.encode(payload, SECRET, algorithm=ALGORITHM)
    with pytest.raises(HTTPException) as exc_info:
        require_auth(_make_credentials(token))
    assert exc_info.value.status_code == 401


def test_tampered_token_raises_401():
    token = _make_token() + "tampered"
    with pytest.raises(HTTPException) as exc_info:
        require_auth(_make_credentials(token))
    assert exc_info.value.status_code == 401
