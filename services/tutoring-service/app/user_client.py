"""Async HTTP client for the User Service learning-profile API.

Fetches and persists the Felder-Silverman dials that live on each user's
learning profile.  All failures are non-fatal: callers receive empty dicts
or False rather than exceptions, so a slow/unavailable user-service never
blocks the chat stream.

Endpoints used:
  GET  /users/{user_id}/profile       → learning_profile.felder_silverman_dials
  PATCH /users/{user_id}/preferences  → { "felder_silverman_dials": {...} }
"""

import logging
from uuid import UUID

import httpx

logger = logging.getLogger(__name__)

_TIMEOUT = httpx.Timeout(2.0)  # 2 s — non-fatal if exceeded


class UserServiceClient:
    """Thin async wrapper around the User Service REST API.

    Instantiated per-request so the bearer token stays scoped to one user.

    Args:
        base_url: Base URL of the user service, e.g. ``http://user-service:8080``.
        bearer_token: JWT extracted from the student's ``Authorization`` header;
            forwarded as-is to authenticate user-service calls.
    """

    def __init__(self, base_url: str, bearer_token: str) -> None:
        """Initialize the client with a service base URL and auth token.

        Args:
            base_url: Base URL of the user service (no trailing slash).
            bearer_token: JWT to forward in the Authorization header.
        """
        self._base_url = base_url.rstrip("/")
        self._headers = {"Authorization": f"Bearer {bearer_token}"}

    async def get_felder_silverman_dials(self, user_id: UUID) -> dict[str, float]:
        """Fetch the student's current Felder-Silverman dials from the User Service.

        Returns an empty dict on any error (404, timeout, connection failure, etc.)
        so the caller can safely fall back to DEFAULT_DIALS without crashing.

        Args:
            user_id: UUID of the student whose profile to fetch.

        Returns:
            Dict mapping dimension name to dial value, e.g.
            ``{"active_reflective": -0.3, "visual_verbal": 0.5, ...}``.
            Empty dict on any failure.
        """
        url = f"{self._base_url}/users/{user_id}/profile"
        try:
            async with httpx.AsyncClient(timeout=_TIMEOUT) as client:
                resp = await client.get(url, headers=self._headers)
            if resp.status_code == 404:
                return {}
            resp.raise_for_status()
            data = resp.json()
            dials = data.get("learning_profile", {}).get("felder_silverman_dials") or {}
            return {k: float(v) for k, v in dials.items()}
        except Exception as exc:  # noqa: BLE001
            logger.warning("get_felder_silverman_dials failed for %s: %s", user_id, exc)
            return {}

    async def patch_felder_silverman_dials(self, user_id: UUID, dials: dict[str, float]) -> bool:
        """Persist updated Felder-Silverman dials to the User Service.

        Sends a PATCH request with the full updated dials dict.  Returns False
        on any failure so the caller can log a warning without crashing.

        Args:
            user_id: UUID of the student whose preferences to update.
            dials: Complete updated dial values to persist.

        Returns:
            True if the server responded with 2xx; False otherwise.
        """
        url = f"{self._base_url}/users/{user_id}/preferences"
        payload = {"felder_silverman_dials": dials}
        try:
            async with httpx.AsyncClient(timeout=_TIMEOUT) as client:
                resp = await client.patch(url, json=payload, headers=self._headers)
            resp.raise_for_status()
            return True
        except Exception as exc:  # noqa: BLE001
            logger.warning("patch_felder_silverman_dials failed for %s: %s", user_id, exc)
            return False
