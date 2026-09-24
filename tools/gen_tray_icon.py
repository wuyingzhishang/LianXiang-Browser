# -*- coding: utf-8 -*-
"""Generate the 链享浏览器 tray icon as a multi-size PNG-compressed ICO."""
from __future__ import annotations

import argparse
import io
import struct
from pathlib import Path

from ant_logo import render_logo


def build_ico(frames):
    pngs = []
    for image in frames:
        buffer = io.BytesIO()
        image.save(buffer, format="PNG", optimize=True)
        pngs.append(buffer.getvalue())

    count = len(pngs)
    header = struct.pack("<HHH", 0, 1, count)
    entries = bytearray()
    body = bytearray()
    offset = 6 + 16 * count
    for png in pngs:
        width = struct.unpack(">I", png[16:20])[0]
        height = struct.unpack(">I", png[20:24])[0]
        entries.extend(
            struct.pack(
                "<BBBBHHII",
                width if width < 256 else 0,
                height if height < 256 else 0,
                0,
                0,
                1,
                32,
                len(png),
                offset,
            )
        )
        body.extend(png)
        offset += len(png)
    return header + bytes(entries) + bytes(body)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--out", default="backend/internal/tray/icon.ico")
    parser.add_argument("--preview", default=None)
    parser.add_argument("--sizes", default="16,20,24,32,40,48,64")
    args = parser.parse_args()

    sizes = [int(value) for value in args.sizes.split(",")]
    frames = [render_logo(size) for size in sizes]
    if args.preview:
        preview_dir = Path(args.preview)
        preview_dir.mkdir(parents=True, exist_ok=True)
        for size, image in zip(sizes, frames):
            image.save(preview_dir / f"tray_{size}.png", format="PNG", optimize=True)

    output = Path(args.out)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_bytes(build_ico(frames))
    print(f"wrote {output}: {output.stat().st_size} bytes, sizes={sizes}")


if __name__ == "__main__":
    main()
