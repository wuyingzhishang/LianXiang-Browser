# -*- coding: utf-8 -*-
"""Shared renderer for the LianXiang Browser application icon.

The source artwork is kept in ``lianxiang_logo_source.png`` so every
platform asset is derived from the same second-party brand mark.  The
renderer deliberately preserves the source alpha channel and uses a
high-quality reduction for small UI sizes.
"""
from __future__ import annotations

from pathlib import Path

from PIL import Image


SOURCE = Path(__file__).with_name("lianxiang_logo_source.png")


def render_logo(size: int = 128) -> Image.Image:
    """Return the LianXiang logo at *size* x *size* in RGBA format."""
    if size < 1:
        raise ValueError("logo size must be positive")
    with Image.open(SOURCE) as source:
        image = source.convert("RGBA")
        if image.size != (size, size):
            image = image.resize((size, size), Image.Resampling.LANCZOS)
        return image.copy()
