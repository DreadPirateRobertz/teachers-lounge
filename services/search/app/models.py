from uuid import UUID

from pydantic import BaseModel, Field


class ChunkResult(BaseModel):
    chunk_id: UUID
    material_id: UUID
    course_id: UUID
    content: str
    score: float
    chapter: str | None = None
    section: str | None = None
    page: int | None = None
    content_type: str = "text"


class SearchResponse(BaseModel):
    query: str
    course_id: UUID
    results: list[ChunkResult]
    total: int
    search_mode: str = Field(default="dense", description="dense | hybrid")
