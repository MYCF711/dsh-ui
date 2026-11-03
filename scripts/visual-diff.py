#!/usr/bin/env python3
"""对同尺寸 PNG 做确定性区域测量和逐像素 diff，只依赖 Python + Pillow。

区域由调用者在 JSON 中用 [x, y, width, height] 明确指定；不使用 OCR、
模型识图、颜色阈值或语义猜测。所有矩形都采用左闭右开坐标。
"""

import argparse
import hashlib
import json
import sys
from functools import reduce
from pathlib import Path

from PIL import Image, ImageChops, ImageStat

SCHEMA_VERSION = 1
TOOL_VERSION = 1


class InputError(ValueError):
    """输入图像或区域契约不合法。"""


def sha256(path):
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def load_image(path):
    try:
        with Image.open(path) as source:
            source.load()
            return {
                "path": str(path),
                "format": source.format,
                "source_mode": source.mode,
                "size": list(source.size),
                "sha256": sha256(path),
            }, source.convert("RGBA")
    except (OSError, ValueError) as exc:
        raise InputError(f"cannot read image {path}: {exc}") from exc


def require_int(value, label):
    if isinstance(value, bool) or not isinstance(value, int):
        raise InputError(f"{label} must be an integer")
    return value


def load_regions(path, image_size):
    try:
        contract = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeError, json.JSONDecodeError) as exc:
        raise InputError(f"cannot read regions {path}: {exc}") from exc
    if not isinstance(contract, dict):
        raise InputError("regions JSON must be an object")
    allowed = {"schema_version", "top_colors", "regions"}
    unknown = sorted(set(contract) - allowed)
    if unknown:
        raise InputError(f"unknown regions JSON keys: {unknown}")
    if contract.get("schema_version") != SCHEMA_VERSION:
        raise InputError(f"schema_version must be {SCHEMA_VERSION}")
    top_colors = contract.get("top_colors", 10)
    require_int(top_colors, "top_colors")
    if not 1 <= top_colors <= 100:
        raise InputError("top_colors must be in [1, 100]")
    specs = contract.get("regions")
    if not isinstance(specs, list) or not specs:
        raise InputError("regions must be a non-empty array")

    width, height = image_size
    regions = []
    names = set()
    for index, spec in enumerate(specs):
        label = f"regions[{index}]"
        if not isinstance(spec, dict) or set(spec) != {"name", "rect"}:
            raise InputError(f"{label} must contain exactly name and rect")
        name = spec["name"]
        if not isinstance(name, str) or not name.strip():
            raise InputError(f"{label}.name must be a non-empty string")
        if name in names:
            raise InputError(f"duplicate region name: {name}")
        names.add(name)
        rect = spec["rect"]
        if not isinstance(rect, list) or len(rect) != 4:
            raise InputError(f"{label}.rect must be [x, y, width, height]")
        x, y, w, h = [require_int(v, f"{label}.rect") for v in rect]
        if x < 0 or y < 0 or w <= 0 or h <= 0 or x + w > width or y + h > height:
            raise InputError(
                f"{label}.rect {rect} is outside image bounds [0, 0, {width}, {height}]"
            )
        regions.append({"name": name, "rect": [x, y, w, h]})
    return top_colors, regions


def color_counts(image):
    colors = image.getcolors(maxcolors=image.width * image.height)
    if colors is None:
        raise InputError("could not enumerate image colors")
    return sorted(colors, key=lambda item: (-item[0], item[1]))


def dominant_on_line(image):
    count, color = color_counts(image)[0]
    return color, count / (image.width * image.height)


def dominant_bands(image, axis):
    """每行/列取精确众数；并列时选 RGBA 字典序最小者，再做 RLE。"""
    width, height = image.size
    line_count = width if axis == "x" else height
    bands = []
    current = None
    for index in range(line_count):
        if axis == "x":
            line = image.crop((index, 0, index + 1, height))
        else:
            line = image.crop((0, index, width, index + 1))
        color, fraction = dominant_on_line(line)
        if current is not None and current["color_rgba"] == list(color):
            current["end"] = index + 1
            current["length"] += 1
            current["min_line_dominant_ratio"] = min(
                current["min_line_dominant_ratio"], fraction
            )
            current["max_line_dominant_ratio"] = max(
                current["max_line_dominant_ratio"], fraction
            )
        else:
            if current is not None:
                bands.append(current)
            current = {
                "start": index,
                "end": index + 1,
                "length": 1,
                "color_rgba": list(color),
                "min_line_dominant_ratio": fraction,
                "max_line_dominant_ratio": fraction,
            }
    if current is not None:
        bands.append(current)
    return {
        "orientation": "vertical" if axis == "x" else "horizontal",
        "line_count": line_count,
        "boundaries": [band["start"] for band in bands[1:]],
        "bands": bands,
    }


def measure_image(image, top_colors):
    width, height = image.size
    pixels = width * height
    colors = color_counts(image)
    return {
        "size": [width, height],
        "pixel_count": pixels,
        "color_count_rgba": len(colors),
        "dominant_colors": [
            {"color_rgba": list(color), "count": count, "ratio": count / pixels}
            for count, color in colors[:top_colors]
        ],
        "dominant_bands": {
            "x": dominant_bands(image, "x"),
            "y": dominant_bands(image, "y"),
        },
    }


def diff_metrics(a, b, origin=(0, 0)):
    width, height = a.size
    pixels = width * height
    delta = ImageChops.difference(a, b)
    # RGBA.getbbox() 默认可能只看 alpha；显式合并四通道，避免漏掉 RGB-only diff。
    mask = reduce(ImageChops.lighter, delta.split())
    bbox = mask.getbbox()
    histogram = mask.histogram()
    different = pixels - histogram[0]
    channel_sums = ImageStat.Stat(delta).sum
    max_delta = max(channel_max for _, channel_max in delta.getextrema())
    ox, oy = origin
    image_bbox = (
        (bbox[0] + ox, bbox[1] + oy, bbox[2] + ox, bbox[3] + oy)
        if bbox is not None else None
    )
    return {
        "size": [width, height],
        "pixel_count": pixels,
        "exact_equal": different == 0,
        "different_pixels": different,
        "difference_ratio": different / pixels,
        "difference_percent": 100 * different / pixels,
        "bbox_ltrb": list(bbox) if bbox is not None else None,
        "bbox_xywh": (
            [bbox[0], bbox[1], bbox[2] - bbox[0], bbox[3] - bbox[1]]
            if bbox is not None else None
        ),
        "image_bbox_ltrb": list(image_bbox) if image_bbox is not None else None,
        "image_bbox_xywh": (
            [image_bbox[0], image_bbox[1],
             image_bbox[2] - image_bbox[0], image_bbox[3] - image_bbox[1]]
            if image_bbox is not None else None
        ),
        "max_channel_delta": max_delta,
        "mean_absolute_channel_delta": sum(channel_sums) / (pixels * 4),
    }


def run(a_path, b_path, regions_path, out_path):
    a_info, a = load_image(a_path)
    b_info, b = load_image(b_path)
    if a.size != b.size:
        raise InputError(f"image sizes differ: {a_info['size']} vs {b_info['size']}")
    top_colors, region_specs = load_regions(regions_path, a.size)

    region_results = []
    for spec in region_specs:
        x, y, w, h = spec["rect"]
        box = (x, y, x + w, y + h)
        a_crop = a.crop(box)
        b_crop = b.crop(box)
        region_results.append({
            "name": spec["name"],
            "rect_xywh": spec["rect"],
            "a": measure_image(a_crop, top_colors),
            "b": measure_image(b_crop, top_colors),
            "diff": diff_metrics(a_crop, b_crop, origin=(x, y)),
        })

    result = {
        "schema_version": SCHEMA_VERSION,
        "tool": {"name": "visual-diff.py", "version": TOOL_VERSION},
        "coordinate_system": {
            "origin": "top_left",
            "rect": "xywh",
            "diff_bbox": "bbox_* is region-local; image_bbox_* is full-image",
            "interval": "left_top_inclusive_right_bottom_exclusive",
            "color": "RGBA",
            "pixel_rule": "any RGBA channel difference counts; no tolerance",
            "band_rule": "per-row/column exact mode; RGBA lexicographically smallest wins ties",
        },
        "inputs": {"a": a_info, "b": b_info},
        "comparison": diff_metrics(a, b),
        "regions": region_results,
    }
    try:
        out_path.parent.mkdir(parents=True, exist_ok=True)
        out_path.write_text(
            json.dumps(result, ensure_ascii=False, indent=2, allow_nan=False) + "\n",
            encoding="utf-8",
        )
    except OSError as exc:
        raise InputError(f"cannot write output {out_path}: {exc}") from exc
    return result


def main(argv=None):
    parser = argparse.ArgumentParser(
        description=__doc__,
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    parser.add_argument("a", type=Path, help="reference image A")
    parser.add_argument("b", type=Path, help="comparison image B; sizes must match A")
    parser.add_argument("--regions", type=Path, required=True,
                        help="JSON with explicit named [x,y,width,height] regions")
    parser.add_argument("--out", type=Path, required=True, help="output JSON path")
    parser.add_argument("--version", action="version", version=f"%(prog)s {TOOL_VERSION}")
    args = parser.parse_args(argv)
    try:
        result = run(args.a, args.b, args.regions, args.out)
    except InputError as exc:
        print(f"visual-diff: error: {exc}", file=sys.stderr)
        return 2
    comparison = result["comparison"]
    print(
        f"OK size={comparison['size'][0]}x{comparison['size'][1]} "
        f"different_pixels={comparison['different_pixels']} "
        f"difference_ratio={comparison['difference_ratio']:.12g} "
        f"bbox_xywh={comparison['bbox_xywh']} out={args.out}"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
