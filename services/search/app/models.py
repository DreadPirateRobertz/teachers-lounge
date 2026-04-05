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


class DiagramResult(BaseModel):
    """A single diagram returned by the diagram search endpoint."""

    diagram_id: str
    course_id: UUID
    gcs_path: str
    caption: str
    figure_type: str = "diagram"  # diagram | chart | table | equation_image
    page: int | None = None
    chapter: str | None = None
    score: float


class DiagramSearchResponse(BaseModel):
    """Response from GET /v1/search/diagrams."""

    query: str
    course_id: UUID
    results: list[DiagramResult]
    total: int
