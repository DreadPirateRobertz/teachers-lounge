"""Tests for the require_auth JWT dependency."""
import pytest
from fastapi import Depends
from fastapi.testclient import TestClient
from jose import jwt

from app.auth import require_auth
from app.config import settings
from app.main import app
from tests.conftest import make_token, TEST_USER_ID


def test_require_auth_valid_token():
    """require_auth returns the sub claim for a well-formed token."""
    token = make_token(TEST_USER_ID)
    from fastapi.security import HTTPAuthorizationCredentials
    cred = HTTPAuthorizationCredentials(scheme="Bearer", credentials=token)
    result = require_auth(cred)
    assert result == TEST_USER_ID


def test_require_auth_missing_credentials():
    """require_auth raises 401 when no credentials are provided."""
    from fastapi import HTTPException
    with pytest.raises(HTTPException) as exc_info:
        require_auth(None)
    assert exc_info.value.status_code == 401


def test_require_auth_invalid_signature():
    """require_auth raises 401 for a token signed with the wrong secret."""
    from fastapi import HTTPException
    from fastapi.security import HTTPAuthorizationCredentials
    bad_token = jwt.encode({"sub": TEST_USER_ID}, "wrong-secret", algorithm="HS256")
    cred = HTTPAuthorizationCredentials(scheme="Bearer", credentials=bad_token)
    with pytest.raises(HTTPException) as exc_info:
        require_auth(cred)
    assert exc_info.value.status_code == 401


def test_require_auth_missing_sub_claim():
    """require_auth raises 401 when the JWT has no sub claim."""
    from fastapi import HTTPException
    from fastapi.security import HTTPAuthorizationCredentials
    token = jwt.encode({"role": "student"}, settings.jwt_secret, algorithm=settings.jwt_algorithm)
    cred = HTTPAuthorizationCredentials(scheme="Bearer", credentials=token)
    with pytest.raises(HTTPException) as exc_info:
        require_auth(cred)
    assert exc_info.value.status_code == 401


def test_require_auth_expired_token():
    """require_auth raises 401 for a token whose exp claim is in the past."""
    import time
    from fastapi import HTTPException
    from fastapi.security import HTTPAuthorizationCredentials

    expired_token = jwt.encode(
        {
            "sub": TEST_USER_ID,
            "aud": settings.jwt_audience,
            "exp": int(time.time()) - 3600,  # 1 hour ago
        },
        settings.jwt_secret,
        algorithm=settings.jwt_algorithm,
    )
    cred = HTTPAuthorizationCredentials(scheme="Bearer", credentials=expired_token)
    with pytest.raises(HTTPException) as exc_info:
        require_auth(cred)
    assert exc_info.value.status_code == 401


def test_require_auth_wrong_audience():
    """require_auth raises 401 for a token signed for a different service audience."""
    from fastapi import HTTPException
    from fastapi.security import HTTPAuthorizationCredentials

    wrong_aud_token = jwt.encode(
        {"sub": TEST_USER_ID, "aud": "some-other-service"},
        settings.jwt_secret,
        algorithm=settings.jwt_algorithm,
    )
    cred = HTTPAuthorizationCredentials(scheme="Bearer", credentials=wrong_aud_token)
    with pytest.raises(HTTPException) as exc_info:
        require_auth(cred)
    assert exc_info.value.status_code == 401
