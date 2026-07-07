from __future__ import annotations

import argparse
import csv
import json
import os
import re
import subprocess
import sys
from pathlib import Path
from typing import Optional


STYLE_SIGNATURE = "CF250_IPHONE_REALPHOTO_V1"
SCHEMA_VERSION = "iphone-product-photo-prompt-manifest/v1"
REQUIRED_FIELDNAMES = ["schema_version", "style_signature", "index", "item_name", "subject_kind", "subject", "prompt", "qa_status"]
FIELDNAMES = [
    "schema_version",
    "style_signature",
    "index",
    "item_name",
    "subject_kind",
    "subject",
    "frontend_card_width",
    "frontend_card_height",
    "asset_width",
    "asset_height",
    "asset_aspect_ratio",
    "photo_role",
    "ui_overlay_required",
    "visual_grade",
    "frame_style",
    "prompt",
    "qa_status",
]
IMAGE_GEN_SCRIPT = Path("C:/Users/admin/.codex/skills/.system/imagegen/scripts/image_gen.py")
DEFAULT_BASE_URL = "https://xaapi.ai/v1"
DEFAULT_IMAGE_MODEL = "gpt-image-2"
DEFAULT_IMAGE_SIZE = "2048x944"
DEFAULT_IMAGE_QUALITY = "medium"
DEFAULT_OUTPUT_FORMAT = "png"
FRONTEND_CARD_WIDTH = 340
FRONTEND_CARD_HEIGHT = 156
DEFAULT_ASSET_WIDTH = 2040
DEFAULT_ASSET_HEIGHT = 936
DEFAULT_ASSET_SIZE = f"{DEFAULT_ASSET_WIDTH}x{DEFAULT_ASSET_HEIGHT}"
VISUAL_GRADES = ("auto", "bronze", "silver", "gold", "diamond", "legendary")
FRAME_STYLES = {
    "bronze": "aged-bronze-market-frame",
    "silver": "brushed-silver-market-frame",
    "gold": "warm-gold-market-frame",
    "diamond": "cool-diamond-prism-frame",
    "legendary": "red-gold-legendary-frame",
}
GRADE_PALETTES = {
    "bronze": {"outer": (126, 74, 35), "mid": (190, 121, 58), "inner": (255, 205, 123), "glow": (255, 143, 54)},
    "silver": {"outer": (94, 105, 116), "mid": (185, 203, 216), "inner": (244, 249, 255), "glow": (160, 220, 255)},
    "gold": {"outer": (121, 76, 18), "mid": (236, 174, 54), "inner": (255, 236, 155), "glow": (255, 196, 60)},
    "diamond": {"outer": (30, 102, 133), "mid": (80, 218, 245), "inner": (238, 255, 255), "glow": (80, 230, 255)},
    "legendary": {"outer": (110, 26, 18), "mid": (246, 70, 42), "inner": (255, 219, 118), "glow": (255, 60, 32)},
}
FIXED_BASE = (
    "Photorealistic image captured on iPhone 17 Pro Max rear camera, f/1.78 aperture, natural lighting, "
    "extremely detailed, sharp focus, authentic smartphone snapshot, zero filters, zero stylization, "
    "true-to-life colors, subtle lens flare if applicable, natural skin texture, real iPhone photography, "
    "4K resolution feel, documentary realism, casual everyday photo"
)

DETAILS = {
    "包子": "fresh steamed bun on plain white breakfast paper, close enough to show soft dough texture and tiny moisture",
    "咖啡": "plain takeaway coffee cup on a real table, close enough to show lid texture and small condensation marks",
    "泡面": "opened instant noodle cup on a kitchen counter, close enough to show noodles and broth surface",
    "打车票": "ride receipt on a car seat, private details blurred by shallow focus",
    "演唱会票": "concert ticket on a plain desk, no real artist name or readable barcode",
    "手机碎屏换新": "cracked smartphone on a real desk, sharp focus on fractured glass",
    "宠物急诊账单": "veterinary emergency deposit receipt on a clinic counter, private information not readable",
    "光明冰砖": "classic rectangular ice cream brick partly unwrapped on plain wrapper paper, cold surface texture visible",
    "国际饭店蝴蝶酥": "butterfly-shaped puff pastry on plain bakery paper, flaky layers sharply visible",
    "摩登红人雪花膏": "small vintage-style face cream jar on a bathroom counter, cream texture and jar surface visible",
    "蜂花檀香皂": "sandalwood soap bar on a plain washstand, waxy soap texture and edges visible",
    "上海英雄钢笔": "fountain pen on a plain desk, nib and glossy barrel in sharp focus",
    "金枫石库门黄酒": "bottle of huangjiu rice wine on an ordinary dining table, glass and amber liquid visible, label not dominant",
    "邵万生黄泥螺": "small open jar of preserved mud snails on a kitchen counter, food texture visible, label not dominant",
    "高桥松饼": "flaky round pastry on plain bakery paper, crisp layered edge visible",
    "乔家栅条头糕": "strip-shaped rice cake on plain parchment paper, soft sticky texture visible",
    "蟹壳黄": "sesame-coated baked pastry on plain paper, crisp golden crust visible",
    "老凤祥金饰": "gold jewelry piece on a plain counter, metal texture and reflection visible, no luxury display staging",
}

DETAIL_KINDS = {
    "包子": "physical-item",
    "咖啡": "physical-item",
    "泡面": "physical-item",
    "打车票": "tangible-token",
    "演唱会票": "tangible-token",
    "手机碎屏换新": "physical-item",
    "宠物急诊账单": "tangible-token",
    "光明冰砖": "physical-item",
    "国际饭店蝴蝶酥": "physical-item",
    "摩登红人雪花膏": "physical-item",
    "蜂花檀香皂": "physical-item",
    "上海英雄钢笔": "physical-item",
    "金枫石库门黄酒": "physical-item",
    "邵万生黄泥螺": "physical-item",
    "高桥松饼": "physical-item",
    "乔家栅条头糕": "physical-item",
    "蟹壳黄": "physical-item",
    "老凤祥金饰": "physical-item",
}

SERVICE_HINTS = (
    "票",
    "券",
    "账单",
    "罚单",
    "发票",
    "收据",
    "押金",
    "补费",
    "手续费",
    "加价",
    "溢价",
    "赔付",
    "退款",
    "急诊",
    "检查",
    "换新",
    "维修",
    "套餐",
    "会员",
    "酒店",
    "打车",
)

BANNED_TERMS = (
    "trading card",
    "dark humor",
    "satirical",
    "consumerist",
    "money bills",
    "red warning",
    "neon",
    "fantasy",
    "poster",
    "concept art",
    "illustration",
    "rendered",
    "card frame",
    "game card",
)

REQUIRED_PHRASES = (
    "Close-up iPhone snapshot",
    "single",
    "landscape horizontal frame for a wide marketplace item tile art crop",
    "centered product silhouette",
    "safe empty margin near all edges for deterministic UI overlay",
    "one dominant item only",
    "casual handheld composition",
    "no text",
    "no border",
    "no decorative frame",
    "no dominant logo",
    "no extra props competing with the subject",
    "Photorealistic image captured on iPhone 17 Pro Max rear camera",
    "zero filters",
    "zero stylization",
    "documentary realism",
)

IMAGE_QA_CHECKS = (
    "one clear main subject",
    "phone-photo look, not studio render or illustration",
    "natural lighting and true-to-life color",
    "no decorative border, poster layout, card frame, floating props, or visual effects",
    "no readable private information on tickets, receipts, bills, or screens",
    "item fills most of the frame while still looking casually photographed",
    "no brand logo dominating the image",
    "no text signature or watermark added by the model",
)


def clean_item(value: str) -> str:
    value = value.strip().lstrip("\ufeff")
    value = re.sub(r"^\s*[\d一二三四五六七八九十]+[.、)]\s*", "", value)
    return re.sub(r"\s+", " ", value)


def validate_item_name(item: str) -> list[str]:
    errors: list[str] = []
    if "\ufffd" in item:
        errors.append("item name contains Unicode replacement characters")
    if "??" in item:
        errors.append("item name contains repeated question marks; check input encoding")
    return errors


def subject_for(item: str) -> str:
    if item in DETAILS:
        return DETAILS[item]
    if any(hint in item for hint in SERVICE_HINTS):
        return f"real receipt or payment slip representing {item}, placed on an ordinary table, no readable private information"
    return f"real {item} placed on an ordinary everyday surface, close enough to show natural material texture"


def subject_kind(item: str) -> str:
    if item in DETAIL_KINDS:
        return DETAIL_KINDS[item]
    if any(hint in item for hint in SERVICE_HINTS):
        return "tangible-token"
    return "physical-item"


def visual_grade_for(item: str, requested: str = "auto") -> str:
    if requested != "auto":
        return requested
    legendary_hints = ("老凤祥", "金饰", "钻石", "黄金", "金条", "房", "车", "奢侈")
    diamond_hints = ("急诊", "手术", "演唱会", "机票", "酒店", "换新", "名表")
    gold_hints = ("国际饭店", "英雄钢笔", "石库门", "黄酒", "邵万生", "雪花膏")
    silver_hints = ("蝴蝶酥", "檀香皂", "松饼", "条头糕", "蟹壳黄", "咖啡")
    if any(hint in item for hint in legendary_hints):
        return "legendary"
    if any(hint in item for hint in diamond_hints):
        return "diamond"
    if any(hint in item for hint in gold_hints):
        return "gold"
    if any(hint in item for hint in silver_hints):
        return "silver"
    return "bronze"


def prompt_for(item: str) -> str:
    return (
        f"Close-up iPhone snapshot of a single {subject_for(item)}, "
        f"landscape horizontal frame for a wide marketplace item tile art crop, centered product silhouette, "
        f"safe empty margin near all edges for deterministic UI overlay, one dominant item only, casual handheld composition, "
        f"no text, no border, no decorative frame, no dominant logo, no extra props competing with the subject, {FIXED_BASE}"
    )


def validate_prompt(prompt: str) -> list[str]:
    errors = [f"missing required phrase: {phrase}" for phrase in REQUIRED_PHRASES if phrase not in prompt]
    lower_prompt = prompt.lower()
    errors.extend(f"banned term present: {term}" for term in BANNED_TERMS if term in lower_prompt)
    if "single single" in lower_prompt:
        errors.append("duplicate wording: single single")
    return errors


def record_for(index: int, item: str, requested_grade: str = "auto") -> dict[str, object]:
    item_errors = validate_item_name(item)
    if item_errors:
        raise ValueError(f"{item}: {'; '.join(item_errors)}")
    prompt = prompt_for(item)
    errors = validate_prompt(prompt)
    if errors:
        raise ValueError(f"{item}: {'; '.join(errors)}")
    visual_grade = visual_grade_for(item, requested_grade)
    return {
        "schema_version": SCHEMA_VERSION,
        "style_signature": STYLE_SIGNATURE,
        "index": index,
        "item_name": item,
        "subject_kind": subject_kind(item),
        "subject": subject_for(item),
        "frontend_card_width": FRONTEND_CARD_WIDTH,
        "frontend_card_height": FRONTEND_CARD_HEIGHT,
        "asset_width": DEFAULT_ASSET_WIDTH,
        "asset_height": DEFAULT_ASSET_HEIGHT,
        "asset_aspect_ratio": f"{FRONTEND_CARD_WIDTH}:{FRONTEND_CARD_HEIGHT}",
        "photo_role": "raw_item_art",
        "ui_overlay_required": True,
        "visual_grade": visual_grade,
        "frame_style": FRAME_STYLES[visual_grade],
        "prompt": prompt,
        "qa_status": "prompt_validated",
    }


def load_manifest(path: Path) -> list[dict[str, object]]:
    if path.suffix.lower() == ".csv":
        with path.open("r", encoding="utf-8-sig", newline="") as handle:
            return list(csv.DictReader(handle))
    records: list[dict[str, object]] = []
    for line_number, line in enumerate(path.read_text(encoding="utf-8-sig").splitlines(), 1):
        if not line.strip():
            continue
        try:
            value = json.loads(line)
        except json.JSONDecodeError as exc:
            raise ValueError(f"line {line_number}: invalid json: {exc}") from exc
        if not isinstance(value, dict):
            raise ValueError(f"line {line_number}: record must be an object")
        records.append(value)
    return records


def validate_record(record: dict[str, object], seen_indexes: set[int]) -> list[str]:
    errors: list[str] = []
    for field in REQUIRED_FIELDNAMES:
        if field not in record or str(record[field]).strip() == "":
            errors.append(f"missing field: {field}")
    if errors:
        return errors
    if record["schema_version"] != SCHEMA_VERSION:
        errors.append(f"schema_version must be {SCHEMA_VERSION}")
    if record["style_signature"] != STYLE_SIGNATURE:
        errors.append(f"style_signature must be {STYLE_SIGNATURE}")
    try:
        index = int(str(record["index"]))
    except ValueError:
        errors.append("index must be an integer")
        index = -1
    if index in seen_indexes:
        errors.append(f"duplicate index: {index}")
    seen_indexes.add(index)
    if record["subject_kind"] not in {"physical-item", "tangible-token"}:
        errors.append("subject_kind must be physical-item or tangible-token")
    if "asset_width" in record and int(str(record["asset_width"])) != DEFAULT_ASSET_WIDTH:
        errors.append(f"asset_width must be {DEFAULT_ASSET_WIDTH}")
    if "asset_height" in record and int(str(record["asset_height"])) != DEFAULT_ASSET_HEIGHT:
        errors.append(f"asset_height must be {DEFAULT_ASSET_HEIGHT}")
    if "visual_grade" in record and str(record["visual_grade"]) not in GRADE_PALETTES:
        errors.append("visual_grade must be bronze, silver, gold, diamond, or legendary")
    if "frame_style" in record and "visual_grade" in record and str(record["frame_style"]) != FRAME_STYLES.get(str(record["visual_grade"])):
        errors.append("frame_style must match visual_grade")
    errors.extend(validate_prompt(str(record["prompt"])))
    if record["qa_status"] != "prompt_validated":
        errors.append("qa_status must be prompt_validated")
    return errors


def validate_manifest(path: Path) -> str:
    records = load_manifest(path)
    if not records:
        raise ValueError("manifest has no records")
    seen_indexes: set[int] = set()
    failures: list[str] = []
    for row_number, record in enumerate(records, 1):
        errors = validate_record(record, seen_indexes)
        if errors:
            item_name = record.get("item_name", "<unknown>")
            failures.append(f"row {row_number} ({item_name}): {'; '.join(errors)}")
    if failures:
        raise ValueError("\n".join(failures))
    return f"Manifest valid: {len(records)} records, style_signature={STYLE_SIGNATURE}"


def write_manifest(records: list[dict[str, object]], path: Path) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text("\n".join(json.dumps(record, ensure_ascii=False) for record in records) + "\n", encoding="utf-8")


def image_output_name(record: dict[str, object], output_format: str) -> str:
    return f"{int(str(record['index'])):04d}.{output_format}"


def write_image2_jobs(records: list[dict[str, object]], path: Path, output_format: str) -> None:
    jobs = []
    for record in records:
        jobs.append(
            {
                "prompt": record["prompt"],
                "out": image_output_name(record, output_format),
                "fields": {
                    "item_name": record["item_name"],
                    "subject_kind": record["subject_kind"],
                    "style_signature": record["style_signature"],
                    "visual_grade": record.get("visual_grade"),
                    "frame_style": record.get("frame_style"),
                },
            }
        )
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text("\n".join(json.dumps(job, ensure_ascii=False) for job in jobs) + "\n", encoding="utf-8")


def read_windows_env(scope: str) -> str:
    try:
        result = subprocess.run(
            [
                "powershell",
                "-NoProfile",
                "-Command",
                f"[Environment]::GetEnvironmentVariable('OPENAI_API_KEY','{scope}')",
            ],
            text=True,
            capture_output=True,
            check=False,
        )
    except Exception:
        return ""
    return result.stdout.strip()


def is_api_key_shape(value: Optional[str]) -> bool:
    if not value or not value.startswith("sk-"):
        return False
    try:
        value.encode("ascii")
    except UnicodeEncodeError:
        return False
    return not bool(re.search(r"\s", value))


def build_child_env(base_url: str) -> dict[str, str]:
    env = os.environ.copy()
    candidates = [
        read_windows_env("User"),
        read_windows_env("Machine"),
        env.get("OPENAI_API_KEY"),
    ]
    for value in candidates:
        if is_api_key_shape(value):
            env["OPENAI_API_KEY"] = value
            break
    env["OPENAI_BASE_URL"] = base_url.rstrip("/")
    return env


def redact_secret(text: str) -> str:
    return re.sub(r"sk-[A-Za-z0-9_*.-]{12,}", "sk-***REDACTED***", text)


def check_image2_channel(args: argparse.Namespace) -> str:
    script = Path(args.image_gen_script or IMAGE_GEN_SCRIPT)
    env = build_child_env(args.base_url)
    status = {
        "image_gen_script": str(script),
        "image_gen_script_exists": script.exists(),
        "model": args.model,
        "base_url": args.base_url.rstrip("/"),
        "openai_api_key_available": is_api_key_shape(env.get("OPENAI_API_KEY")),
    }
    return json.dumps(status, ensure_ascii=False, indent=2)


def parse_size(value: str) -> tuple[int, int]:
    match = re.fullmatch(r"([1-9][0-9]*)x([1-9][0-9]*)", value.strip())
    if not match:
        raise ValueError("size must use WIDTHxHEIGHT, for example 2040x936")
    return int(match.group(1)), int(match.group(2))


def resize_cover(image, target_width: int, target_height: int):
    from PIL import ImageOps

    image = ImageOps.exif_transpose(image.convert("RGB"))
    scale = max(target_width / image.width, target_height / image.height)
    resized_width = max(target_width, round(image.width * scale))
    resized_height = max(target_height, round(image.height * scale))
    resized = image.resize((resized_width, resized_height))
    left = max(0, (resized_width - target_width) // 2)
    top = max(0, (resized_height - target_height) // 2)
    return resized.crop((left, top, left + target_width, top + target_height))


def normalize_image_dir(images_dir: Path, asset_size: str, output_format: str) -> list[str]:
    from PIL import Image

    target_width, target_height = parse_size(asset_size)
    suffix = "." + output_format.lower()
    normalized: list[str] = []
    for path in sorted(images_dir.glob(f"*{suffix}")):
        with Image.open(path) as image:
            original = f"{image.width}x{image.height}"
            fixed = resize_cover(image, target_width, target_height)
            fixed.save(path)
        normalized.append(f"{path.name}: {original} -> {target_width}x{target_height}")
    if not normalized:
        raise FileNotFoundError(f"no {suffix} images found in {images_dir}")
    return normalized


def validate_image_dir(images_dir: Path, asset_size: str, output_format: str) -> str:
    from PIL import Image

    target_width, target_height = parse_size(asset_size)
    suffix = "." + output_format.lower()
    failures: list[str] = []
    count = 0
    for path in sorted(images_dir.glob(f"*{suffix}")):
        count += 1
        with Image.open(path) as image:
            if image.width != target_width or image.height != target_height:
                failures.append(f"{path.name}: {image.width}x{image.height}, want {target_width}x{target_height}")
    if count == 0:
        raise FileNotFoundError(f"no {suffix} images found in {images_dir}")
    if failures:
        raise ValueError("\n".join(failures))
    return f"Images valid: {count} files, size={target_width}x{target_height}"


def grade_from_record(record: dict[str, object]) -> str:
    raw = str(record.get("visual_grade") or "").strip().lower()
    if raw in GRADE_PALETTES:
        return raw
    return visual_grade_for(str(record.get("item_name", "")), "auto")


def draw_diamond(draw, cx: int, cy: int, radius: int, fill: tuple[int, int, int, int], outline: tuple[int, int, int, int]) -> None:
    points = [(cx, cy - radius), (cx + radius, cy), (cx, cy + radius), (cx - radius, cy)]
    draw.polygon(points, fill=fill, outline=outline)


def apply_card_frame(image, record: dict[str, object], asset_size: str):
    from PIL import Image, ImageDraw, ImageFilter

    target_width, target_height = parse_size(asset_size)
    grade = grade_from_record(record)
    palette = GRADE_PALETTES[grade]
    base = resize_cover(image, target_width, target_height).convert("RGBA")

    overlay = Image.new("RGBA", base.size, (0, 0, 0, 0))
    draw = ImageDraw.Draw(overlay)

    # Quiet dark shelves for later title/price UI without asking the model to draw UI.
    draw.rounded_rectangle((34, 28, target_width - 34, 150), radius=34, fill=(18, 14, 10, 112))
    draw.rounded_rectangle((34, target_height - 150, target_width - 34, target_height - 28), radius=34, fill=(18, 14, 10, 132))

    glow = Image.new("RGBA", base.size, (0, 0, 0, 0))
    glow_draw = ImageDraw.Draw(glow)
    glow_color = (*palette["glow"], 130)
    glow_draw.rounded_rectangle((30, 22, target_width - 30, target_height - 22), radius=44, outline=glow_color, width=26)
    glow = glow.filter(ImageFilter.GaussianBlur(16))
    overlay = Image.alpha_composite(overlay, glow)
    draw = ImageDraw.Draw(overlay)

    draw.rounded_rectangle((18, 14, target_width - 18, target_height - 14), radius=42, outline=(*palette["outer"], 255), width=30)
    draw.rounded_rectangle((46, 40, target_width - 46, target_height - 40), radius=30, outline=(*palette["mid"], 235), width=14)
    draw.rounded_rectangle((68, 62, target_width - 68, target_height - 62), radius=22, outline=(*palette["inner"], 185), width=5)

    corner = 138
    for sx, sy in ((1, 1), (-1, 1), (1, -1), (-1, -1)):
        x0 = 18 if sx > 0 else target_width - 18
        y0 = 14 if sy > 0 else target_height - 14
        plate = [
            (x0, y0),
            (x0 + sx * corner, y0),
            (x0 + sx * (corner - 36), y0 + sy * 38),
            (x0 + sx * 38, y0 + sy * (corner - 36)),
            (x0, y0 + sy * corner),
        ]
        draw.polygon(plate, fill=(*palette["outer"], 210), outline=(*palette["inner"], 210))

    rank = {"bronze": 1, "silver": 2, "gold": 3, "diamond": 4, "legendary": 5}[grade]
    start_x = target_width - 92 - (rank - 1) * 42
    y = target_height - 82
    for i in range(rank):
        draw_diamond(draw, start_x + i * 42, y, 16, (*palette["inner"], 230), (*palette["outer"], 255))

    if grade == "legendary":
        draw.rounded_rectangle((82, 76, target_width - 82, target_height - 76), radius=18, outline=(255, 70, 34, 115), width=6)
    if grade == "diamond":
        draw.rounded_rectangle((82, 76, target_width - 82, target_height - 76), radius=18, outline=(110, 240, 255, 130), width=6)

    return Image.alpha_composite(base, overlay)


def render_card_frame_dir(images_dir: Path, frame_out: Path, records: list[dict[str, object]], asset_size: str, output_format: str) -> list[str]:
    from PIL import Image

    frame_out.mkdir(parents=True, exist_ok=True)
    record_by_index = {}
    for record in records:
        try:
            record_by_index[int(str(record["index"]))] = record
        except Exception:
            continue
    rendered: list[str] = []
    suffix = "." + output_format.lower()
    for position, path in enumerate(sorted(images_dir.glob(f"*{suffix}")), 1):
        try:
            index = int(path.stem)
        except ValueError:
            index = position
        record = record_by_index.get(index, {"index": index, "item_name": path.stem, "visual_grade": "bronze"})
        with Image.open(path) as image:
            framed = apply_card_frame(image, record, asset_size)
            output = frame_out / path.name
            if output_format == "png":
                framed.save(output)
            else:
                framed.convert("RGB").save(output)
        rendered.append(f"{path.name}: {grade_from_record(record)} -> {output}")
    if not rendered:
        raise FileNotFoundError(f"no {suffix} images found in {images_dir}")
    return rendered


def run_image2_batch(records: list[dict[str, object]], args: argparse.Namespace) -> None:
    script = Path(args.image_gen_script or IMAGE_GEN_SCRIPT)
    if not script.exists():
        raise FileNotFoundError(f"image_gen.py not found: {script}")
    out_dir = Path(args.image2_out)
    images_dir = out_dir / "images"
    manifest_path = out_dir / "prompt_manifest.jsonl"
    jobs_path = out_dir / "image2_jobs.jsonl"
    write_manifest(records, manifest_path)
    write_image2_jobs(records, jobs_path, args.output_format)
    command = [
        sys.executable,
        str(script),
        "generate-batch",
        "--input",
        str(jobs_path),
        "--out-dir",
        str(images_dir),
        "--model",
        args.model,
        "--size",
        args.size,
        "--quality",
        args.quality,
        "--output-format",
        args.output_format,
        "--concurrency",
        str(args.concurrency),
        "--max-attempts",
        str(args.max_attempts),
        "--force",
        "--no-augment",
    ]
    if args.dry_run:
        command.append("--dry-run")
    result = subprocess.run(command, text=True, capture_output=True, env=build_child_env(args.base_url))
    if result.stdout:
        print(redact_secret(result.stdout), end="")
    if result.stderr:
        print(redact_secret(result.stderr), end="", file=sys.stderr)
    if result.returncode != 0:
        raise RuntimeError(f"image2 batch failed with exit code {result.returncode}")
    if not args.dry_run and not args.no_normalize_images:
        for line in normalize_image_dir(images_dir, args.asset_size, args.output_format):
            print(f"normalized: {line}")
    if not args.dry_run and not args.no_render_card_frames:
        frame_out = Path(args.frame_out) if args.frame_out else out_dir / "framed"
        for line in render_card_frame_dir(images_dir, frame_out, records, args.asset_size, args.output_format):
            print(f"framed: {line}")
    print(f"manifest: {manifest_path}")
    print(f"image2_jobs: {jobs_path}")
    print(f"images: {images_dir}")
    if not args.dry_run and not args.no_render_card_frames:
        print(f"framed: {Path(args.frame_out) if args.frame_out else out_dir / 'framed'}")


def read_items(args: argparse.Namespace) -> list[str]:
    raw: list[str] = []
    if args.file:
        raw.extend(Path(args.file).read_text(encoding="utf-8").splitlines())
    if args.items:
        raw.extend(args.items)
    if not raw and not sys.stdin.isatty():
        raw.extend(sys.stdin.read().splitlines())
    items = [clean_item(item) for item in raw]
    return [item for item in items if item]


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser()
    parser.add_argument("--validate-manifest", help="Validate an existing JSONL or CSV prompt manifest")
    parser.add_argument("--manifest", help="Use an existing JSONL or CSV prompt manifest")
    parser.add_argument("--print-image-qa", action="store_true", help="Print the finished-image QA checklist")
    parser.add_argument("--check-image2-channel", action="store_true", help="Print image2 channel status without generating")
    parser.add_argument("--validate-images", help="Validate all images in a directory against --asset-size")
    parser.add_argument("--normalize-images", help="Resize/crop all images in a directory to --asset-size in place")
    parser.add_argument("--render-card-frames", help="Render deterministic game-card frame previews from an image directory")
    parser.add_argument("--frame-out", help="Output directory for framed card previews")
    parser.add_argument("--image2-out", help="Generate images with gpt-image-2 into this output directory")
    parser.add_argument("--image-gen-script", help="Path to image_gen.py")
    parser.add_argument("--base-url", default=DEFAULT_BASE_URL)
    parser.add_argument("--model", default=DEFAULT_IMAGE_MODEL)
    parser.add_argument("--size", default=DEFAULT_IMAGE_SIZE)
    parser.add_argument("--quality", default=DEFAULT_IMAGE_QUALITY)
    parser.add_argument("--output-format", choices=("png", "jpeg", "webp"), default=DEFAULT_OUTPUT_FORMAT)
    parser.add_argument("--asset-size", default=DEFAULT_ASSET_SIZE)
    parser.add_argument("--no-normalize-images", action="store_true")
    parser.add_argument("--no-render-card-frames", action="store_true")
    parser.add_argument("--visual-grade", choices=VISUAL_GRADES, default="auto")
    parser.add_argument("--concurrency", type=int, default=2)
    parser.add_argument("--max-attempts", type=int, default=3)
    parser.add_argument("--dry-run", action="store_true")
    parser.add_argument("--file", help="UTF-8 text file, one item per line")
    parser.add_argument("--items", nargs="*", help="Item names")
    parser.add_argument("--format", choices=("list", "jsonl", "csv"), default="list", help="Output format")
    parser.add_argument("--out", help="Write output to a file instead of stdout")
    return parser


def main() -> None:
    args = build_parser().parse_args()
    if args.print_image_qa:
        print("\n".join(f"- {check}" for check in IMAGE_QA_CHECKS))
        return
    if args.check_image2_channel:
        print(check_image2_channel(args))
        return
    if args.validate_images:
        print(validate_image_dir(Path(args.validate_images), args.asset_size, args.output_format))
        return
    if args.normalize_images:
        for line in normalize_image_dir(Path(args.normalize_images), args.asset_size, args.output_format):
            print(f"normalized: {line}")
        print(validate_image_dir(Path(args.normalize_images), args.asset_size, args.output_format))
        return
    if args.render_card_frames:
        records = load_manifest(Path(args.manifest)) if args.manifest else []
        frame_out = Path(args.frame_out) if args.frame_out else Path(args.render_card_frames).parent / "framed"
        for line in render_card_frame_dir(Path(args.render_card_frames), frame_out, records, args.asset_size, args.output_format):
            print(f"framed: {line}")
        return
    if args.validate_manifest:
        print(validate_manifest(Path(args.validate_manifest)))
        return
    if args.manifest:
        records = load_manifest(Path(args.manifest))
        seen_indexes: set[int] = set()
        failures = []
        for row_number, record in enumerate(records, 1):
            errors = validate_record(record, seen_indexes)
            if errors:
                failures.append(f"row {row_number}: {'; '.join(errors)}")
        if failures:
            raise ValueError("\n".join(failures))
    else:
        items = read_items(args)
        if not items:
            raise SystemExit("No items provided.")
        records = [record_for(index, item, args.visual_grade) for index, item in enumerate(items, 1)]
    if args.image2_out:
        run_image2_batch(records, args)
        return
    if args.format == "jsonl":
        output = "\n".join(json.dumps(record, ensure_ascii=False) for record in records)
    elif args.format == "csv":
        writer_target = sys.stdout if not args.out else None
        if writer_target is not None:
            writer = csv.DictWriter(writer_target, fieldnames=FIELDNAMES)
            writer.writeheader()
            writer.writerows(records)
            return
        output_path = Path(args.out)
        with output_path.open("w", encoding="utf-8-sig", newline="") as handle:
            writer = csv.DictWriter(handle, fieldnames=FIELDNAMES)
            writer.writeheader()
            writer.writerows(records)
        return
    else:
        output = "\n".join(f"{record['index']}. {record['prompt']}" for record in records)
    if args.out:
        Path(args.out).write_text(output + "\n", encoding="utf-8")
        return
    print(output)


if __name__ == "__main__":
    main()
