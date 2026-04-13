"""FERPA audit log writer for the Tutoring Service.

The tutoring service shares the same Postgres database as user-service,
so it writes directly to the audit_log table rather than calling the
user-service API (which would create a circular dependency on the hot path).

Actions logged:
  READ_INTERACTIONS  — student fetches their own conversation history
  READ_QUIZ_RESULTS  — (placeholder; quiz results are in user-service)
"""

import logging
from typing import Optional
from uuid import UUID

from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession

logger = logging.getLogger(__name__)

# FERPA audit action constants (mirror models.go constants in user-service).
ACTION_READ_INTERACTIONS = "READ_INTERACTIONS"
ACTION_READ_QUIZ_RESULTS = "READ_QUIZ_RESULTS"
ACTION_READ_PROFILE = "READ_PROFILE"


async def write_audit_log(
    db: AsyncSession,
    *,
    accessor_id: Optional[UUID],
    student_id: Optional[UUID],
    action: str,
    data_accessed: str,
    purpose: str,
    ip_address: str = "",
) -> None:
    """Write one row to the shared audit_log table.

    Failures are logged and swallowed — audit logging must never block the
    tutoring stream or raise an HTTP error to the student.

    Args:
        db: Active SQLAlchemy async session.
        accessor_id: UUID of the user making the request (may equal student_id).
        student_id: UUID of the student whose data is being accessed.
        action: One of the FERPA action constants (e.g. READ_INTERACTIONS).
        data_accessed: Human-readable description of the data touched.
        purpose: Reason for the access (e.g. "user_request").
        ip_address: Client IP for audit trail; empty string if unavailable.
    """
    try:
        await db.execute(
            text(
                "INSERT INTO audit_log "
                "(accessor_id, student_id, action, data_accessed, purpose, ip_address) "
                "VALUES (:accessor_id, :student_id, :action, :data_accessed, :purpose, "
                "        :ip_address::inet)"
            ),
            {
                "accessor_id": str(accessor_id) if accessor_id else None,
                "student_id": str(student_id) if student_id else None,
                "action": action,
                "data_accessed": data_accessed,
                "purpose": purpose,
                "ip_address": ip_address or None,
            },
        )
        await db.commit()
    except Exception as exc:  # noqa: BLE001
        logger.warning("audit_log write failed: %s", exc)
