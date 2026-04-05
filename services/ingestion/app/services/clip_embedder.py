"""CLIP image embedding for diagram ingestion (Phase 6).

Encodes extracted figure images into 768-d CLIP vectors using
``openai/clip-vit-base-patch32``.  Falls back to deterministic random
unit vectors when transformers/torch are not installed so the rest of the
ingestion pipeline can run in lightweight environments.
"""
import hashlib
import logging
import math
import random
from pathlib import Path

from app.config import settings

logger = logging.getLogger(__name__)

_model = None
_processor = None
_clip_available: bool | None = None


def _probe_clip() -> bool:
    """Return True if transformers + torch + Pillow are importable."""
    global _clip_available
    if _clip_available is not None:
        return _clip_available
    try:
        import torch  # noqa: F401
        from PIL import Image  # noqa: F401
        from transformers import CLIPModel, CLIPProcessor  # noqa: F401
        _clip_available = True
    except ImportError:
        logger.warning("transformers/torch/Pillow not installed — using random CLIP stub")
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
    _model.training = False
    logger.info("CLIP model loaded")
    return _model, _processor


def _random_unit_vector(seed: int) -> list[float]:
    rng = random.Random(seed)
    raw = [rng.gauss(0, 1) for _ in range(settings.clip_embedding_dim)]
    norm = math.sqrt(sum(x * x for x in raw))
    return [x / norm for x in raw]


def embed_image_sync(image_path: Path) -> list[float]:
    """Embed a local image file into a 768-d CLIP unit vector (synchronous).

    Called via run_in_executor so the event loop is not blocked.  Uses the
    transformers CLIP vision encoder when available; otherwise returns a
    deterministic stub vector derived from the file path.

    Args:
        image_path: Local path to the extracted figure image (PNG/JPEG).

    Returns:
        768-d float list normalised to unit length.
    """
    if not _probe_clip():
        seed = int(hashlib.md5(str(image_path).encode()).hexdigest(), 16) % (2**31)
        return _random_unit_vector(seed)

    import torch
    from PIL import Image

    model, processor = _get_model_and_processor()
    image = Image.open(image_path).convert("RGB")
    inputs = processor(images=image, return_tensors="pt")
    with torch.no_grad():
        features = model.get_image_features(**inputs)
        features = features / features.norm(dim=-1, keepdim=True)
    return features[0].tolist()


async def embed_image(image_path: Path) -> list[float]:
    """Async wrapper around embed_image_sync — runs in executor.

    Args:
        image_path: Local path to the figure image.

    Returns:
        768-d float list normalised to unit length.
    """
    import asyncio
    loop = asyncio.get_running_loop()
    return await loop.run_in_executor(None, embed_image_sync, image_path)
