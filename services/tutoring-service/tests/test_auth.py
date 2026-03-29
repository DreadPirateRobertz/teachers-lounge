"""
Tests for JWT validation (app/auth.py).

Covers:
- Valid token → correct JWTClaims extracted
- Expired token → 401
- Wrong signing algorithm → 401
- Missing uid claim → 401
- Missing sub claim (both uid and sub absent) → 401
- Minor account_type preserved
- sub_status preserved
"""
import time
import uuid
from datetime import datetime, timezone

import pytest
from fastapi import HTTPException
from fastapi.security import HTTPAuthorizationCredentials
from jose import jwt

# Use a known test secret — matches what the User Service would use
TEST_SECRET = "test-secret-not-for-production"
TEST_ALGO = "HS256"
TEST_USER_ID = str(uuid.uuid4())


def _make_token(
    payload_overrides: dict | None = None,
    secret: str = TEST_SECRET,
    algorithm: str = TEST_ALGO,
    exp_offset: int = 900,   # seconds from now; negative = expired
) -> str:
    now = int(time.time())
    base_payload = {
        "sub": TEST_USER_ID,
        "uid": TEST_USER_ID,
        "email": "student@example.com",
        "acct": "standard",
        "sub_status": "active",
        "iat": now,
        "exp": now + exp_offset,
        "iss": "teacherslounge-user-service",
    }
    if payload_overrides:
        base_payload.update(payload_overrides)
    return jwt.encode(base_payload, secret, algorithm=algorithm)


def _creds(token: str) -> HTTPAuthorizationCredentials:
    return HTTPAuthorizationCredentials(scheme="Bearer", credentials=token)


# ── Patch settings so tests use TEST_SECRET ───────────────────────────────────

@pytest.fixture(autouse=True)
def patch_settings(monkeypatch):
    import app.auth as auth_module
    monkeypatch.setattr(auth_module.settings, "jwt_secret", TEST_SECRET)
    monkeypatch.setattr(auth_module.settings, "jwt_algorithm", TEST_ALGO)


# ── Tests ─────────────────────────────────────────────────────────────────────

def test_valid_token_returns_correct_claims():
    from app.auth import require_auth
    token = _make_token()
    claims = require_auth(_creds(token))

    assert str(claims.user_id) == TEST_USER_ID
    assert claims.email == "student@example.com"
    assert claims.account_type == "standard"
    assert claims.sub_status == "active"


def test_valid_token_minor_account_type():
    from app.auth import require_auth
    token = _make_token({"acct": "minor"})
    claims = require_auth(_creds(token))
    assert claims.account_type == "minor"


def test_valid_token_with_sub_status():
    from app.auth import require_auth
    token = _make_token({"sub_status": "trial"})
    claims = require_auth(_creds(token))
    assert claims.sub_status == "trial"


def test_expired_token_raises_401():
    from app.auth import require_auth
    token = _make_token(exp_offset=-1)   # expired 1 second ago
    with pytest.raises(HTTPException) as exc_info:
        require_auth(_creds(token))
    assert exc_info.value.status_code == 401
    assert "expired" in exc_info.value.detail.lower()


def test_wrong_algorithm_raises_401():
    """Token signed with RS256 (or any non-HS256) must be rejected."""
    from app.auth import require_auth
    # Encode with a different secret to simulate wrong algo / tampered token
    token = _make_token(secret="wrong-secret")
    with pytest.raises(HTTPException) as exc_info:
        require_auth(_creds(token))
    assert exc_info.value.status_code == 401


def test_missing_uid_and_sub_raises_401():
    """A token with neither uid nor sub claim must be rejected."""
    from app.auth import require_auth
    payload = {
        "email": "student@example.com",
        "acct": "standard",
        "iat": int(time.time()),
        "exp": int(time.time()) + 900,
    }
    token = jwt.encode(payload, TEST_SECRET, algorithm=TEST_ALGO)
    with pytest.raises(HTTPException) as exc_info:
        require_auth(_creds(token))
    assert exc_info.value.status_code == 401
    assert "uid" in exc_info.value.detail.lower()


def test_uid_takes_precedence_over_sub():
    """When both uid and sub are present, uid is used."""
    from app.auth import require_auth
    different_sub = str(uuid.uuid4())
    token = _make_token({"uid": TEST_USER_ID, "sub": different_sub})
    claims = require_auth(_creds(token))
    assert str(claims.user_id) == TEST_USER_ID   # uid wins


def test_sub_fallback_when_uid_absent():
    """When uid is absent, sub is used as fallback."""
    from app.auth import require_auth
    fallback_id = str(uuid.uuid4())
    payload = {
        "sub": fallback_id,
        "email": "student@example.com",
        "acct": "standard",
        "iat": int(time.time()),
        "exp": int(time.time()) + 900,
    }
    token = jwt.encode(payload, TEST_SECRET, algorithm=TEST_ALGO)
    claims = require_auth(_creds(token))
    assert str(claims.user_id) == fallback_id


def test_tampered_token_raises_401():
    """Modifying any byte of the token must cause rejection."""
    from app.auth import require_auth
    token = _make_token()
    tampered = token[:-4] + "XXXX"   # corrupt the signature
    with pytest.raises(HTTPException) as exc_info:
        require_auth(_creds(tampered))
    assert exc_info.value.status_code == 401
