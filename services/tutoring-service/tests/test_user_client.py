"""
Unit tests for app.user_client.UserServiceClient.

Uses respx to mock outbound HTTP without a live server.

Covers:
  get_felder_silverman_dials  — 200 with dials, 200 missing dials, 404, timeout, conn error
  patch_felder_silverman_dials — 200, 204, 500, timeout
  Bearer token forwarded on every request
"""

import uuid

import httpx
import pytest
import respx

from app.user_client import UserServiceClient

BASE = "http://user-service:8080"
TOKEN = "test-jwt-token"
USER_ID = uuid.uuid4()


@pytest.fixture()
def client():
    """Return a UserServiceClient pointed at BASE with a test token."""
    return UserServiceClient(base_url=BASE, bearer_token=TOKEN)


# ── get_felder_silverman_dials ────────────────────────────────────────────────


@respx.mock
@pytest.mark.asyncio
async def test_get_dials_returns_dials_on_200(client):
    """200 response with nested dials returns them as a float dict."""
    respx.get(f"{BASE}/users/{USER_ID}/profile").mock(
        return_value=httpx.Response(
            200,
            json={
                "learning_profile": {
                    "felder_silverman_dials": {
                        "active_reflective": "-0.3",
                        "visual_verbal": "0.5",
                    }
                }
            },
        )
    )
    result = await client.get_felder_silverman_dials(USER_ID)
    assert result == {"active_reflective": -0.3, "visual_verbal": 0.5}


@respx.mock
@pytest.mark.asyncio
async def test_get_dials_returns_empty_on_404(client):
    """404 response returns an empty dict (user has no profile yet)."""
    respx.get(f"{BASE}/users/{USER_ID}/profile").mock(return_value=httpx.Response(404))
    result = await client.get_felder_silverman_dials(USER_ID)
    assert result == {}


@respx.mock
@pytest.mark.asyncio
async def test_get_dials_returns_empty_on_500(client):
    """5xx response returns an empty dict (non-fatal)."""
    respx.get(f"{BASE}/users/{USER_ID}/profile").mock(return_value=httpx.Response(500))
    result = await client.get_felder_silverman_dials(USER_ID)
    assert result == {}


@respx.mock
@pytest.mark.asyncio
async def test_get_dials_returns_empty_on_connection_error(client):
    """Connection error returns an empty dict (non-fatal)."""
    respx.get(f"{BASE}/users/{USER_ID}/profile").mock(side_effect=httpx.ConnectError("refused"))
    result = await client.get_felder_silverman_dials(USER_ID)
    assert result == {}


@respx.mock
@pytest.mark.asyncio
async def test_get_dials_returns_empty_on_timeout(client):
    """Read timeout returns an empty dict (non-fatal)."""
    respx.get(f"{BASE}/users/{USER_ID}/profile").mock(side_effect=httpx.ReadTimeout("timed out"))
    result = await client.get_felder_silverman_dials(USER_ID)
    assert result == {}


@respx.mock
@pytest.mark.asyncio
async def test_get_dials_returns_empty_when_profile_missing_from_body(client):
    """200 response missing learning_profile key returns an empty dict."""
    respx.get(f"{BASE}/users/{USER_ID}/profile").mock(
        return_value=httpx.Response(200, json={"id": str(USER_ID)})
    )
    result = await client.get_felder_silverman_dials(USER_ID)
    assert result == {}


@respx.mock
@pytest.mark.asyncio
async def test_get_dials_forwards_bearer_token(client):
    """Authorization header is forwarded verbatim to the user service."""
    route = respx.get(f"{BASE}/users/{USER_ID}/profile").mock(
        return_value=httpx.Response(200, json={"learning_profile": {"felder_silverman_dials": {}}})
    )
    await client.get_felder_silverman_dials(USER_ID)
    assert route.called
    assert route.calls[0].request.headers["authorization"] == f"Bearer {TOKEN}"


# ── patch_felder_silverman_dials ──────────────────────────────────────────────


@respx.mock
@pytest.mark.asyncio
async def test_patch_dials_returns_true_on_200(client):
    """200 OK response returns True."""
    respx.patch(f"{BASE}/users/{USER_ID}/preferences").mock(return_value=httpx.Response(200))
    result = await client.patch_felder_silverman_dials(USER_ID, {"visual_verbal": -0.3})
    assert result is True


@respx.mock
@pytest.mark.asyncio
async def test_patch_dials_returns_true_on_204(client):
    """204 No Content is also a success."""
    respx.patch(f"{BASE}/users/{USER_ID}/preferences").mock(return_value=httpx.Response(204))
    result = await client.patch_felder_silverman_dials(USER_ID, {})
    assert result is True


@respx.mock
@pytest.mark.asyncio
async def test_patch_dials_returns_false_on_500(client):
    """Server error returns False (non-fatal)."""
    respx.patch(f"{BASE}/users/{USER_ID}/preferences").mock(return_value=httpx.Response(500))
    result = await client.patch_felder_silverman_dials(USER_ID, {"visual_verbal": 0.1})
    assert result is False


@respx.mock
@pytest.mark.asyncio
async def test_patch_dials_returns_false_on_timeout(client):
    """Timeout returns False (non-fatal)."""
    respx.patch(f"{BASE}/users/{USER_ID}/preferences").mock(
        side_effect=httpx.ReadTimeout("timed out")
    )
    result = await client.patch_felder_silverman_dials(USER_ID, {})
    assert result is False


@respx.mock
@pytest.mark.asyncio
async def test_patch_dials_sends_correct_body(client):
    """PATCH request body contains felder_silverman_dials key with the provided dials."""
    import json

    route = respx.patch(f"{BASE}/users/{USER_ID}/preferences").mock(
        return_value=httpx.Response(200)
    )
    dials = {"active_reflective": 0.15, "visual_verbal": -0.3}
    await client.patch_felder_silverman_dials(USER_ID, dials)

    assert route.called
    body = json.loads(route.calls[0].request.content)
    assert body == {"felder_silverman_dials": dials}


@respx.mock
@pytest.mark.asyncio
async def test_patch_dials_forwards_bearer_token(client):
    """Authorization header is forwarded to the user service on PATCH."""
    route = respx.patch(f"{BASE}/users/{USER_ID}/preferences").mock(
        return_value=httpx.Response(200)
    )
    await client.patch_felder_silverman_dials(USER_ID, {})
    assert route.calls[0].request.headers["authorization"] == f"Bearer {TOKEN}"
