"""CLIP text embedding for diagram retrieval (Phase 6).

Encodes text queries into 768-d CLIP vectors using
``openai/clip-vit-base-patch32``.  When the ``transformers`` / ``torch``
packages are not installed (CI, lightweight dev containers) the module falls
back to a deterministic random unit vector so the rest of the service can boot
and tests can run without GPU or large model weights.
"""
import hashlib
import logging
import math
import random
from typing import TYPE_CHECKING

from app.config import settings

if TYPE_CHECKING:
    pass

logger = logging.getLogger(__name__)

_model = None
_processor = None
_clip_available: bool | None = None  # None = not yet probed


def _probe_clip() -> bool:
    """Return True if transformers + torch are importable."""
    global _clip_available
    if _clip_available is not None:
        return _clip_available
    try:
        import torch  # noqa: F401
        from transformers import CLIPModel, CLIPProcessor  # noqa: F401
        _clip_available = True
    except ImportError:
        logger.warning("transformers/torch not installed — using random CLIP stub")
        _clip_available = False
    return _clip_available


def _get_model_and_processor():
    """Lazily load CLIP model and processor (cached across calls)."""
    global _model, _processor
    if _model is not None:
        return _model, _processor
    from transformers import CLIPModel, CLIPProcessor
    logger.info("Loading CLIP model %s …", settings.clip_model)
    _processor = CLIPProcessor.from_pretrained(settings.clip_model)
    _model = CLIPModel.from_pretrained(settings.clip_model)
    # Set to inference mode (no dropout, no batch-norm tracking)
    _model.training = False
    logger.info("CLIP model loaded")
    return _model, _processor


def _random_unit_vector(seed: int) -> list[float]:
    rng = random.Random(seed)
    raw = [rng.gauss(0, 1) for _ in range(settings.clip_embedding_dim)]
    norm = math.sqrt(sum(x * x for x in raw))
    return [x / norm for x in raw]


async def embed_text_clip(text: str) -> list[float]:
    """Embed *text* into a 768-d CLIP unit vector.

    Uses ``openai/clip-vit-base-patch32`` when transformers is available;
    otherwise returns a deterministic stub vector (same input → same vector,
    useful for unit-test assertions).

    Args:
        text: The query string to embed.

    Returns:
        768-d float list normalised to unit length.
    """
    if not _probe_clip():
        seed = int(hashlib.md5(text.encode()).hexdigest(), 16) % (2**31)
        return _random_unit_vector(seed)

    import asyncio
    import torch

    loop = asyncio.get_running_loop()

    def _encode() -> list[float]:
        model, processor = _get_model_and_processor()
        inputs = processor(text=[text], return_tensors="pt", padding=True, truncation=True)
        with torch.no_grad():
            features = model.get_text_features(**inputs)
            features = features / features.norm(dim=-1, keepdim=True)
        return features[0].tolist()

    return await loop.run_in_executor(None, _encode)
