#!/usr/bin/env python3
"""Merender slide video pelajaran menjadi berkas PNG (dipakai generate-media.sh).

Setiap pelajaran terdiri dari tiga slide: judul, ringkasan, dan latihan.
Renderer ini memakai Pillow (hanya dibutuhkan saat membuat aset, bukan saat
aplikasi berjalan).

Pemakaian:
    python3 scripts/render_media_slides.py <direktori-keluaran> <spesifikasi.json>

Berkas spesifikasi berisi array objek:
    [{"name": "numerik-deret", "slides": [["Judul", "sub"], ["baris1", "baris2"], ...]}]
"""

import json
import os
import sys
import textwrap

from PIL import Image, ImageDraw, ImageFont

WIDTH, HEIGHT = 1280, 720
NAVY = (11, 31, 58)
BRAND = (40, 100, 255)
FLAME = (255, 138, 61)
WHITE = (255, 255, 255)
MIST = (167, 181, 200)

FONT_CANDIDATES = [
    "/System/Library/Fonts/Supplemental/Arial Bold.ttf",
    "/System/Library/Fonts/Supplemental/Arial.ttf",
    "/System/Library/Fonts/Helvetica.ttc",
    "/Library/Fonts/Arial.ttf",
    "/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
    "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
]


def load_font(size: int) -> ImageFont.FreeTypeFont:
    for path in FONT_CANDIDATES:
        if os.path.exists(path):
            try:
                return ImageFont.truetype(path, size)
            except OSError:
                continue
    raise SystemExit(
        "Font TrueType tidak ditemukan. Pasang font sistem (mis. DejaVu Sans) lalu ulangi."
    )


def draw_centered(
    draw: ImageDraw.ImageDraw,
    lines: list[str],
    font: ImageFont.FreeTypeFont,
    top: int,
    fill=WHITE,
    spacing: int = 14,
) -> int:
    """Menggambar baris teks terpusat; mengembalikan tinggi total."""
    heights = [draw.textbbox((0, 0), line, font=font)[3] for line in lines]
    total = sum(heights) + spacing * (len(lines) - 1)
    y = top
    for line, height in zip(lines, heights):
        width = draw.textbbox((0, 0), line, font=font)[2]
        draw.text(((WIDTH - width) / 2, y), line, font=font, fill=fill)
        y += height + spacing
    return total


def render_slide(title: str, body: list[str], index: int, total: int, badge: str) -> Image.Image:
    image = Image.new("RGB", (WIDTH, HEIGHT), NAVY)
    draw = ImageDraw.Draw(image)

    # Batang aksen di atas + garis tipis di bawah.
    draw.rectangle([0, 0, WIDTH, 10], fill=BRAND)
    draw.rectangle([0, HEIGHT - 6, WIDTH, HEIGHT], fill=FLAME)

    # Lencana modul.
    badge_font = load_font(26)
    badge_width = draw.textbbox((0, 0), badge, font=badge_font)[2]
    draw.rounded_rectangle(
        [(WIDTH - badge_width) / 2 - 24, 64, (WIDTH + badge_width) / 2 + 24, 116],
        radius=26,
        outline=BRAND,
        width=2,
    )
    draw.text(((WIDTH - badge_width) / 2, 74), badge, font=badge_font, fill=(143, 180, 255))

    if index == 0:
        title_font = load_font(64)
        lines = textwrap.wrap(title, width=24) or [title]
        draw_centered(draw, lines, title_font, 250)
    else:
        heading_font = load_font(44)
        heading = textwrap.wrap(title, width=34) or [title]
        draw_centered(draw, heading, heading_font, 170)

        body_font = load_font(32)
        wrapped: list[str] = []
        for paragraph in body:
            wrapped.extend(textwrap.wrap(paragraph, width=52) or [paragraph])
            wrapped.append("")
        wrapped = [line for line in wrapped if line != ""] if len(wrapped) == 1 else wrapped
        draw_centered(draw, [line for line in wrapped if line], body_font, 300, fill=MIST, spacing=16)

    footer_font = load_font(24)
    footer = "BUMN Academy · Ruang Kelas"
    draw.text((64, HEIGHT - 60), footer, font=footer_font, fill=(120, 140, 170))

    counter = f"{index + 1} / {total}"
    counter_width = draw.textbbox((0, 0), counter, font=footer_font)[2]
    draw.text((WIDTH - counter_width - 64, HEIGHT - 60), counter, font=footer_font, fill=(120, 140, 170))

    return image


def main() -> None:
    if len(sys.argv) < 3:
        raise SystemExit(__doc__)

    out_dir, spec_path = sys.argv[1], sys.argv[2]
    with open(spec_path, encoding="utf-8") as handle:
        spec = json.load(handle)

    os.makedirs(out_dir, exist_ok=True)

    for item in spec:
        name = item["name"]
        badge = item.get("badge", "Ruang Kelas")
        slides = item["slides"]

        for index, slide in enumerate(slides):
            title = slide[0]
            body = slide[1:]
            image = render_slide(title, body, index, len(slides), badge)
            image.save(os.path.join(out_dir, f"{name}-{index + 1}.png"))

        # Poster (thumbnail) dari slide pertama untuk daftar pelajaran.
        poster = render_slide(slides[0][0], [], 0, len(slides), badge)
        poster.save(os.path.join(out_dir, f"{name}-poster.jpg"), quality=86)

        print(f"  • {name}: {len(slides)} slide + poster")


if __name__ == "__main__":
    main()
