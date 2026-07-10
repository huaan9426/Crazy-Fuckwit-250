from __future__ import annotations

import argparse
import csv
import hashlib
import json
import os
import re
import subprocess
import sys
from pathlib import Path
from typing import Optional


LEGACY_STYLE_SIGNATURE = "CF250_IPHONE_REALPHOTO_V1"
LEGACY_MIXED_STYLE_SET_SIGNATURE = "CF250_ITEM_ART_MIXED_V2"
STYLE_SET_SIGNATURE = "CF250_ITEM_ART_COHESIVE_V3"
ART_DIRECTION_SIGNATURE = "CF250_GAME_ITEM_ART_BIBLE_V1"
LEGACY_SCHEMA_VERSION = "iphone-product-photo-prompt-manifest/v1"
SCHEMA_VERSION = "item-art-prompt-manifest/v2"
STYLE_ASSIGNMENT = "stable-sha256-v1"
DEFAULT_STYLE_SEED = "cf250-item-art-v1"
LEGACY_REQUIRED_FIELDNAMES = ["schema_version", "style_signature", "index", "item_name", "subject_kind", "subject", "prompt", "qa_status"]
V2_REQUIRED_FIELDNAMES = LEGACY_REQUIRED_FIELDNAMES + ["subject_category", "style_profile", "style_family", "style_assignment", "style_seed"]
FIELDNAMES = [
    "schema_version",
    "style_signature",
    "index",
    "item_name",
    "subject_kind",
    "subject",
    "subject_category",
    "style_profile",
    "style_family",
    "style_assignment",
    "style_seed",
    "art_direction_signature",
    "visual_archetype",
    "palette_profile",
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
STYLE_PROFILE_ORDER = (
    "fauvist-paint",
    "pop-art-print",
    "constructivist-poster",
    "baroque-still-life",
    "hyperreal-ad",
    "retro-east-asian-ad",
)
STYLE_CHOICES = ("auto", "iphone-realphoto", *STYLE_PROFILE_ORDER)
MASTER_ITEM_ART_DIRECTION = (
    "Unified premium game item illustration with cinematic commercial realism and a polished hand-painted finish. "
    "Present the product as the only hero in a dynamic three-quarter or strong diagonal view, with a complete readable silhouette, "
    "crisp material highlights, a localized luminous key and rim light, readable midtones, and open shadow detail. "
    "Use subtle lens perspective for impact without warping product geometry. Build the background from two broad color masses "
    "and one shallow environmental plane, keeping all visual energy directed toward the product"
)
MASTER_SCENE_ART_DIRECTION = (
    "Unified premium game service-scene illustration with cinematic advertising realism and a polished hand-painted finish. "
    "Present one immediately readable experience, place, or action as the sole visual focus in a wide environmental composition, "
    "using dynamic perspective, a clear foreground-midground-background hierarchy, localized key and rim light, readable midtones, "
    "and open shadow detail. Allow only the people and supporting objects essential to explain the service, keep faces non-identifying, "
    "and build the scene from two broad color masses without crowds, montages, split panels, or unrelated activities"
)
STYLE_PROFILES = {
    "fauvist-paint": {
        "family": "fauvist-commercial-painting",
        "signature": f"{STYLE_SET_SIGNATURE}/fauvist-paint",
        "prompt": (
            "Fauvist-inspired commercial painting treatment with broad confident brushwork in the background and reflected light, "
            "expressive complementary-color shadows, while silhouette-critical edges and product materials remain precise"
        ),
    },
    "pop-art-print": {
        "family": "pop-art-commercial-print",
        "signature": f"{STYLE_SET_SIGNATURE}/pop-art-print",
        "prompt": (
            "Pop Art advertising treatment with selective halftone dots, tactile silkscreen ink texture, and one restrained dark contour, "
            "applied mainly to the background while the product keeps dimensional volume and credible materials"
        ),
    },
    "constructivist-poster": {
        "family": "constructivist-consumer-poster",
        "signature": f"{STYLE_SET_SIGNATURE}/constructivist-poster",
        "prompt": (
            "Constructivist-inspired commercial treatment with two or three large diagonal background planes and restrained screen-print texture, "
            "while the product remains dimensional, materially convincing, and free of poster typography"
        ),
    },
    "baroque-still-life": {
        "family": "baroque-product-still-life",
        "signature": f"{STYLE_SET_SIGNATURE}/baroque-still-life",
        "prompt": (
            "Baroque still-life treatment with rich oil glazing, jewel-tone depth, and one warm directional beam, "
            "but with raised readable shadows and no crushed black areas at thumbnail size"
        ),
    },
    "hyperreal-ad": {
        "family": "contemporary-hyperreal-advertising",
        "signature": f"{STYLE_SET_SIGNATURE}/hyperreal-ad",
        "prompt": (
            "Hyperreal contemporary advertising treatment with a physical paper set, low three-quarter close-up, hard directional flash, "
            "brilliant controlled highlights, and dense but readable realistic shadow"
        ),
    },
    "retro-east-asian-ad": {
        "family": "retro-east-asian-magazine-ad",
        "signature": f"{STYLE_SET_SIGNATURE}/retro-east-asian-ad",
        "prompt": (
            "1990s East Asian magazine-ad treatment with an airbrushed gouache and product-photography hybrid finish, "
            "bright frontal commercial light, broad geometric color fields, and subtle offset-print grain"
        ),
    },
}
LEGACY_MIXED_STYLE_ANCHORS = {
    "fauvist-paint": "Fauvist-inspired commercial product painting",
    "pop-art-print": "Classic Pop Art advertising print",
    "constructivist-poster": "Constructivist-inspired consumer poster",
    "baroque-still-life": "Baroque still-life oil painting",
    "hyperreal-ad": "Hyperreal contemporary product advertising photography",
    "retro-east-asian-ad": "1990s East Asian magazine advertisement",
}
PALETTE_PROFILE_ORDER = (
    "ember-cobalt",
    "tropical-daylight",
    "candy-electric",
    "jade-gold",
    "citrus-steel",
    "plum-mint",
)
PALETTE_PROFILES = {
    "ember-cobalt": "cobalt blue and warm amber with a restrained scarlet accent",
    "tropical-daylight": "turquoise and leaf green with a restrained sunflower-yellow accent",
    "candy-electric": "coral pink and clean cyan with a restrained violet accent",
    "jade-gold": "deep jade and warm ivory with a restrained vermilion accent",
    "citrus-steel": "steel blue and citrus yellow with a restrained orange accent",
    "plum-mint": "deep plum and pale mint with a restrained rose accent",
}
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

CATEGORY_CUES = {
    "food": "preserve edible structure and emphasize fresh surface texture with at most one natural appetite cue such as steam, glaze, or crisp crumbs, without garnish or extra ingredients",
    "drink": "preserve the real container and liquid behavior, showing glass, condensation, foam, or translucency only when physically plausible",
    "drinkware": "preserve the authentic vessel silhouette, lid, handle, straw, finish, and scale; do not invent openings, emblems, or accessories",
    "personal-care": "preserve packaging proportions and show convincing glass, cream, soap, wax, or plastic material",
    "stationery": "preserve functional geometry and emphasize nib, barrel, paper, wood, or metal detail",
    "jewelry": "preserve scale and construction while showing controlled metal reflections and fine surface detail",
    "electronics": "preserve exact device geometry, port placement, controls, and screen shape where present while showing convincing glass, metal, plastic, wear, or damage",
    "appliance": "preserve the authentic appliance footprint, controls, vents, integral cable, and moving parts where present without inventing functions, sensors, or attachments",
    "cookware": "preserve the authentic cookware profile, handles, lid, enamel, metal, ceramic, and manufacturing details without added food or utensils",
    "apparel": "preserve the complete wearable silhouette and show fabric, stitching, leather, or rubber texture",
    "ticket": "keep one ticket or pass recognizable by shape and layout while all printed details remain abstract and unreadable",
    "bill-or-receipt": "keep one bill, invoice, or receipt recognizable by shape and layout with no readable personal or payment data",
    "service-token": "show one tangible token that clearly represents the service while all private details remain unreadable",
    "service-scene": "show the real experience, place, or action itself rather than a voucher, receipt, email, membership card, or confirmation screen",
    "general-product": "preserve real-world proportions, defining silhouette, function, and material texture",
}

VISUAL_ARCHETYPE_CUES = {
    "food-closeup": "use a close low three-quarter view; the edible surface is the focal point and the full identity remains obvious",
    "tall-vessel": "place the vessel on a strong diagonal or three-quarter axis, keeping its full lid, handle, base, and brand-defining outline visible",
    "handheld-electronics": "use a three-quarter hero angle that exposes the real screen, controls, ports, and thickness without impossible floating parts",
    "appliance": "use a grounded three-quarter hero view with the full footprint visible and one restrained energy accent behind the product",
    "cookware": "use a low three-quarter view that clearly separates body, lid, rim, handles, and material finish",
    "soft-apparel": "show the complete wearable silhouette with one controlled flowing fold or lifted edge, no body, mannequin, hanger, or extra garment",
    "flat-document": "show the complete single document at a slight perspective angle with clean outer edges and abstract unreadable printing",
    "experience-scene": "use the full wide canvas for one immediately recognizable service moment with one dominant focal cluster and limited supporting context",
    "small-hard-good": "use a macro three-quarter hero view with precise construction, tactile surface detail, and a clean complete silhouette",
    "hero-product": "use a grounded three-quarter hero view with the complete defining silhouette and function immediately readable",
}

STYLE_POOLS_BY_CATEGORY = {
    "ticket": ("pop-art-print", "constructivist-poster", "hyperreal-ad", "retro-east-asian-ad"),
    "bill-or-receipt": ("pop-art-print", "constructivist-poster", "hyperreal-ad", "retro-east-asian-ad"),
    "service-token": ("pop-art-print", "constructivist-poster", "hyperreal-ad", "retro-east-asian-ad"),
}

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

SERVICE_SCENE_DETAILS = {
    "钱柜KTV（包厢预订）": "one vivid private-room KTV experience centered on a glowing karaoke display and a single microphone in a premium empty room ready for guests, no booking document or readable lyrics",
    "ChatGPT Plus 订阅": "one modern creator workspace centered on a laptop displaying an abstract AI conversation interface, one seated user silhouette optional, no confirmation email, voucher, receipt, or readable screen text",
    "Keep健身App年卡": "one energetic fitness-app training moment centered on a single athlete running on a treadmill with one nearby phone showing abstract workout rings, no membership card or readable interface text",
    "携程租车（自驾游SUV）": "one modern SUV as the clear hero driving through a broad scenic mountain road, communicating a self-drive rental journey without any reservation document or readable road sign",
    "盒马鲜生（生鲜配送）": "one fresh-grocery delivery moment centered on a chilled delivery tote at a bright apartment doorway, seafood and produce limited to supporting context, no order receipt or readable branding",
    "叮咚买菜": "one doorstep grocery-delivery moment centered on a courier handing over a single vegetable-filled delivery bag, limited supporting produce, no order receipt or readable branding",
    "摄影工作室（婚纱照套系）": "one cinematic wedding portrait session in a professional studio, a bride and groom forming one focal pair under softbox light with a camera silhouette in the foreground, no booking voucher or readable text",
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

DOCUMENT_HINTS = (
    "票",
    "券",
    "账单",
    "罚单",
    "发票",
    "收据",
    "押金",
    "缴费",
    "费用",
    "补费",
    "手续费",
    "加价",
    "溢价",
    "赔付",
    "退款",
    "invoice",
    "receipt",
    "bill",
    "ticket",
    "boarding pass",
)

SERVICE_SCENE_HINTS = (
    "预订",
    "预约",
    "订阅",
    "年卡",
    "会员",
    "套餐",
    "套系",
    "配送",
    "外卖",
    "租车",
    "自驾",
    "摄影",
    "婚纱照",
    "健身",
    "买菜",
    "酒店",
    "打车",
    "急诊",
    "检查",
    "维修",
    "换新",
    "保洁",
    "美容",
    "理发",
    "按摩",
    "课程",
    "培训",
    "旅行",
    "旅游",
    "快递",
    "service",
    "subscription",
    "membership",
    "booking",
    "rental",
    "delivery",
    "fitness",
    "photography",
)

LEGACY_BANNED_TERMS = (
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

LEGACY_REQUIRED_PHRASES = (
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

COMMON_BANNED_TERMS = (
    "trading card",
    "dark humor",
    "satirical",
    "consumerist",
    "money bills",
    "floating money",
    "red warning",
    "warning neon",
    "title bar",
    "price tag",
    "stat box",
)

MIXED_REQUIRED_PHRASES = (
    "Single-item game UI artwork of a single",
    "Wide horizontal 340:156 item-card art composition",
    "one dominant item only",
    "narrow 6 percent safe edge margin",
    "Preserve the item's real-world identity",
    "no duplicate item",
    "no card border",
    "no game UI",
    "no watermark",
)

SCENE_REQUIRED_PHRASES = (
    "Single-image game UI artwork for",
    "Wide horizontal 340:156 item-card art composition",
    "one dominant visual focus only",
    "full wide canvas",
    "narrow 6 percent safe edge margin",
    "one coherent scene",
    "No voucher",
    "No montage",
    "no card border",
    "no game UI",
    "no watermark",
)

COHESIVE_ITEM_REQUIRED_PHRASES = (
    "Unified premium game item illustration",
    "two broad color masses",
    "preserving the product's authentic body color",
    "match its known silhouette and hardware layout",
)

COHESIVE_SCENE_REQUIRED_PHRASES = (
    "Unified premium game service-scene illustration",
    "foreground-midground-background hierarchy",
    "two broad color masses",
    "faces non-identifying",
)

LEGACY_IMAGE_QA_CHECKS = (
    "one clear main subject",
    "phone-photo look, not studio render or illustration",
    "natural lighting and true-to-life color",
    "no decorative border, poster layout, card frame, floating props, or visual effects",
    "no readable private information on tickets, receipts, bills, or screens",
    "item fills most of the frame while still looking casually photographed",
    "no brand logo dominating the image",
    "no text signature or watermark added by the model",
)

IMAGE_QA_CHECKS = (
    "one clear physical item or one coherent service scene, never a collage",
    "physical item occupies about 70 to 88 percent, or a service focal cluster occupies about 55 to 72 percent, and remains recognizable at 340x156",
    "physical goods preserve authentic geometry and materials; service scenes show the actual experience rather than a voucher or email unless explicitly requested",
    "service scenes contain only essential people and supporting objects, with non-identifying faces and no crowd",
    "selected style treatment stays subordinate to item or service identity",
    "two broad background color masses, one restrained accent, readable midtones, and open shadow detail",
    "localized light and motion energy direct attention toward the item or service focal cluster rather than the background",
    "no card border, game UI, rarity badge, or scene clutter; physical-item images contain no people or hands",
    "no readable private data, invented typography, signature, or watermark",
    "exact final pixel size 2040x936",
    "reviewed in a true-size 340x156 contact sheet beside adjacent batch items",
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


def has_hint(item: str, hints: tuple[str, ...]) -> bool:
    normalized = item.casefold()
    return any(hint.casefold() in normalized for hint in hints)


def subject_for(item: str) -> str:
    if item in DETAILS:
        return DETAILS[item]
    if item in SERVICE_SCENE_DETAILS:
        return SERVICE_SCENE_DETAILS[item]
    if has_hint(item, DOCUMENT_HINTS):
        if has_hint(item, ("票", "券", "ticket", "boarding pass")):
            return f"one real ticket or pass representing {item}, shown alone with a complete outline and no readable barcode or private information"
        return f"real receipt or payment slip representing {item}, placed on an ordinary table, no readable private information"
    if has_hint(item, SERVICE_SCENE_HINTS):
        return f"one cinematic real-world service experience representing {item}, centered on one immediately recognizable place or action with limited supporting context"
    return f"named real-world product {item}, shown alone with its authentic market shape and defining material texture"


def subject_kind(item: str) -> str:
    if item in DETAIL_KINDS:
        return DETAIL_KINDS[item]
    if has_hint(item, DOCUMENT_HINTS):
        return "tangible-token"
    if item in SERVICE_SCENE_DETAILS or has_hint(item, SERVICE_SCENE_HINTS):
        return "experience-scene"
    return "physical-item"


def subject_category_for(item: str) -> str:
    normalized = item.casefold()
    if any(hint in normalized for hint in ("账单", "罚单", "发票", "收据", "押金", "缴费", "费用", "invoice", "receipt", "bill")):
        return "bill-or-receipt"
    if any(hint in normalized for hint in ("票", "券", "通行证", "登机牌", "ticket", "boarding pass")):
        return "ticket"
    if subject_kind(item) == "tangible-token":
        return "service-token"
    if subject_kind(item) == "experience-scene":
        return "service-scene"
    if any(
        hint in normalized
        for hint in (
            "咖啡机",
            "胶囊机",
            "吹风机",
            "扫地机器人",
            "吸尘器",
            "电饭煲",
            "冰箱",
            "洗衣机",
            "微波炉",
            "空调",
            "烤箱",
            "洗碗机",
            "净化器",
            "榨汁机",
            "破壁机",
            "料理机",
            "烤面包机",
            "电风扇",
            "取暖器",
            "空气炸锅",
            "coffee machine",
            "capsule machine",
            "hair dryer",
            "robot vacuum",
            "vacuum cleaner",
            "air fryer",
            "refrigerator",
            "washing machine",
            "microwave",
            "air conditioner",
            "dishwasher",
            "appliance",
        )
    ):
        return "appliance"
    if any(hint in normalized for hint in ("保温杯", "水杯", "马克杯", "随行杯", "酒杯", "茶杯", "咖啡杯", "杯子", "杯", "保温壶", "水壶", "tumbler", "thermos", "travel mug", "water bottle", "mug")):
        return "drinkware"
    if any(hint in normalized for hint in ("铸铁锅", "平底锅", "炒锅", "汤锅", "炖锅", "砂锅", "煎锅", "锅具", "锅", "cast iron pot", "dutch oven", "frying pan", "saucepan", "cookware")):
        return "cookware"
    if any(hint in normalized for hint in ("咖啡", "酒", "茶", "饮料", "可乐", "果汁", "矿泉水", "牛奶", "酸奶", "coffee", "wine", "tea", "beverage", "juice", "milk")):
        return "drink"
    if any(hint in normalized for hint in ("香皂", "雪花膏", "面霜", "面膜", "香水", "口红", "洗发", "护肤", "牙膏", "化妆", "face cream", "moisturizer")):
        return "personal-care"
    if any(hint in normalized for hint in ("手机", "电脑", "耳机", "相机", "电视", "手表", "充电", "键盘", "鼠标", "游戏机", "平板", "显示器", "路由器", "音箱", "电器", "airpods", "earbuds", "nintendo switch", "power bank", "smartphone", "laptop", "headphones", "camera", "keyboard", "mouse", "console", "tablet", "monitor", "router", "speaker")):
        return "electronics"
    if any(hint in normalized for hint in ("金饰", "首饰", "项链", "戒指", "手镯", "耳环", "钻石", "黄金", "珠宝")):
        return "jewelry"
    if any(hint in normalized for hint in ("钢笔", "铅笔", "圆珠笔", "文具", "笔记本", "墨水", "尺子")):
        return "stationery"
    if any(hint in normalized for hint in ("包子", "糕", "饼", "酥", "面包", "泡面", "黄泥螺", "糖", "巧克力", "冰淇淋", "冰砖", "零食", "炒饭", "米饭", "白粥", "粥品", "bun", "pastry", "bread", "noodle", "chocolate", "ice cream")):
        return "food"
    if any(hint in normalized for hint in ("衣服", "外套", "衬衫", "裤", "鞋", "帽", "背包", "手提包", "皮带", "shirt", "jacket", "coat", "trousers", "pants", "shoes", "hat", "backpack", "handbag", "belt")):
        return "apparel"
    return "general-product"


def visual_archetype_for(item: str) -> str:
    category = subject_category_for(item)
    if category in {"ticket", "bill-or-receipt", "service-token"}:
        return "flat-document"
    if category == "service-scene":
        return "experience-scene"
    if category == "food":
        return "food-closeup"
    if category in {"drink", "drinkware"}:
        return "tall-vessel"
    if category == "electronics":
        return "handheld-electronics"
    if category == "appliance":
        return "appliance"
    if category == "cookware":
        return "cookware"
    if category == "apparel":
        return "soft-apparel"
    if category in {"personal-care", "stationery", "jewelry"}:
        return "small-hard-good"
    return "hero-product"


def style_profile_for(item: str, requested: str = "auto", style_seed: str = DEFAULT_STYLE_SEED) -> str:
    if requested != "auto":
        if requested != "iphone-realphoto" and requested not in STYLE_PROFILES:
            raise ValueError(f"unknown style profile: {requested}")
        return requested
    if not style_seed:
        raise ValueError("style_seed must not be empty")
    pool = STYLE_POOLS_BY_CATEGORY.get(subject_category_for(item), STYLE_PROFILE_ORDER)
    digest = hashlib.sha256(f"{style_seed}\0{item}".encode("utf-8")).digest()
    return pool[int.from_bytes(digest[:8], "big") % len(pool)]


def palette_profile_for(item: str, style_seed: str = DEFAULT_STYLE_SEED) -> str:
    if not style_seed:
        raise ValueError("style_seed must not be empty")
    digest = hashlib.sha256(f"palette\0{style_seed}\0{item}".encode("utf-8")).digest()
    return PALETTE_PROFILE_ORDER[int.from_bytes(digest[:8], "big") % len(PALETTE_PROFILE_ORDER)]


def composition_cue_for(item: str) -> str:
    archetype = visual_archetype_for(item)
    if archetype == "flat-document":
        return "the single flat token occupies 70 to 82 percent of the frame width at a slight perspective angle while its full outline stays visible"
    if archetype == "experience-scene":
        return "the dominant action or focal cluster occupies 55 to 72 percent of the frame while the environment uses the full width and remains readable at thumbnail size"
    if archetype in {"tall-vessel", "handheld-electronics", "small-hard-good"}:
        return "the item's long axis runs diagonally across the frame and occupies 76 to 88 percent of the usable long dimension while the full identity stays readable"
    if archetype == "soft-apparel":
        return "the complete wearable silhouette occupies 68 to 80 percent of the usable frame without folding into an unrecognizable shape"
    return "the item fills 78 to 86 percent of the usable image area while its complete identifying silhouette stays visible"


def style_signature_for(style_profile: str) -> str:
    if style_profile == "iphone-realphoto":
        return LEGACY_STYLE_SIGNATURE
    return str(STYLE_PROFILES[style_profile]["signature"])


def style_signature_is_compatible(style_profile: str, signature: str) -> bool:
    return signature in {
        style_signature_for(style_profile),
        f"{LEGACY_MIXED_STYLE_SET_SIGNATURE}/{style_profile}",
    }


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


def prompt_for(item: str, requested_style: str = "auto", style_seed: str = DEFAULT_STYLE_SEED) -> str:
    style_profile = style_profile_for(item, requested_style, style_seed)
    if style_profile == "iphone-realphoto":
        return (
            f"Close-up iPhone snapshot of a single {subject_for(item)}, "
            f"landscape horizontal frame for a wide marketplace item tile art crop, centered product silhouette, "
            f"safe empty margin near all edges for deterministic UI overlay, one dominant item only, casual handheld composition, "
            f"no text, no border, no decorative frame, no dominant logo, no extra props competing with the subject, {FIXED_BASE}"
        )
    category = subject_category_for(item)
    archetype = visual_archetype_for(item)
    palette = palette_profile_for(item, style_seed)
    if archetype == "experience-scene":
        return (
            f"Single-image game UI artwork for {subject_for(item)}, {CATEGORY_CUES[category]}. "
            f"Wide horizontal 340:156 item-card art composition using the full wide canvas, one dominant visual focus only, {composition_cue_for(item)}, "
            f"{VISUAL_ARCHETYPE_CUES[archetype]}. {MASTER_SCENE_ART_DIRECTION}. "
            f"Use {PALETTE_PROFILES[palette]} mainly in environmental color and light accents while preserving plausible local colors. "
            f"Apply this controlled surface treatment: {STYLE_PROFILES[style_profile]['prompt']}. "
            f"Keep a narrow 6 percent safe edge margin and one coherent scene. No voucher, receipt, invoice, email, membership card, confirmation page, or document unless explicitly named by the input. "
            f"No montage, no collage, no split panel, no crowd, no unrelated secondary action, no readable personal data, no added typography, "
            f"no card border, no game UI, no rarity badge, no signature, no watermark"
        )
    return (
        f"Single-item game UI artwork of a single {subject_for(item)}, {CATEGORY_CUES[category]}. "
        f"Wide horizontal 340:156 item-card art composition, one dominant item only, {composition_cue_for(item)}, "
        f"{VISUAL_ARCHETYPE_CUES[archetype]}. {MASTER_ITEM_ART_DIRECTION}. "
        f"Use {PALETTE_PROFILES[palette]} mainly in the background and light accents, preserving the product's authentic body color. "
        f"Apply this controlled surface treatment: {STYLE_PROFILES[style_profile]['prompt']}. "
        f"Keep a narrow 6 percent safe edge margin. Preserve the item's real-world identity, proportions, function, and defining material cues. "
        f"When a brand or model is named, match its known silhouette and hardware layout; omit uncertain logos or printed details instead of inventing them. "
        f"Exactly one item, no duplicate item, no competing props, no people, no hands, no added typography, unavoidable printed marks abstract and unreadable, "
        f"no readable personal data, no dominant logo, no card border, no game UI, no rarity badge, no signature, no watermark"
    )


def validate_prompt(
    prompt: str,
    style_profile: str = "iphone-realphoto",
    require_cohesive: bool = False,
    visual_archetype: Optional[str] = None,
) -> list[str]:
    if style_profile == "iphone-realphoto":
        errors = [f"missing required phrase: {phrase}" for phrase in LEGACY_REQUIRED_PHRASES if phrase not in prompt]
        banned_terms = LEGACY_BANNED_TERMS
    elif style_profile in STYLE_PROFILES:
        required_phrases = SCENE_REQUIRED_PHRASES if visual_archetype == "experience-scene" else MIXED_REQUIRED_PHRASES
        errors = [f"missing required phrase: {phrase}" for phrase in required_phrases if phrase not in prompt]
        if require_cohesive:
            cohesive_phrases = COHESIVE_SCENE_REQUIRED_PHRASES if visual_archetype == "experience-scene" else COHESIVE_ITEM_REQUIRED_PHRASES
            errors.extend(f"missing required phrase: {phrase}" for phrase in cohesive_phrases if phrase not in prompt)
        anchors = (
            str(STYLE_PROFILES[style_profile]["prompt"]).split(" with ", 1)[0],
            LEGACY_MIXED_STYLE_ANCHORS[style_profile],
        )
        if not any(anchor in prompt for anchor in anchors):
            errors.append(f"missing style anchor: {anchors[0]}")
        banned_terms = COMMON_BANNED_TERMS
    else:
        return [f"unknown style profile: {style_profile}"]
    lower_prompt = prompt.lower()
    errors.extend(f"banned term present: {term}" for term in banned_terms if term in lower_prompt)
    if "single single" in lower_prompt:
        errors.append("duplicate wording: single single")
    return errors


def record_for(
    index: int,
    item: str,
    requested_grade: str = "auto",
    requested_style: str = "auto",
    style_seed: str = DEFAULT_STYLE_SEED,
) -> dict[str, object]:
    item_errors = validate_item_name(item)
    if item_errors:
        raise ValueError(f"{item}: {'; '.join(item_errors)}")
    style_profile = style_profile_for(item, requested_style, style_seed)
    prompt = prompt_for(item, style_profile, style_seed)
    archetype = visual_archetype_for(item)
    errors = validate_prompt(prompt, style_profile, style_profile != "iphone-realphoto", archetype)
    if errors:
        raise ValueError(f"{item}: {'; '.join(errors)}")
    visual_grade = visual_grade_for(item, requested_grade)
    record = {
        "schema_version": LEGACY_SCHEMA_VERSION if style_profile == "iphone-realphoto" else SCHEMA_VERSION,
        "style_signature": style_signature_for(style_profile),
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
    if style_profile != "iphone-realphoto":
        record.update(
            {
                "subject_category": subject_category_for(item),
                "style_profile": style_profile,
                "style_family": STYLE_PROFILES[style_profile]["family"],
                "style_assignment": STYLE_ASSIGNMENT if requested_style == "auto" else "explicit",
                "style_seed": style_seed,
                "art_direction_signature": ART_DIRECTION_SIGNATURE,
                "visual_archetype": archetype,
                "palette_profile": palette_profile_for(item, style_seed),
            }
        )
    return record


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
    schema_version = str(record.get("schema_version", ""))
    required_fields = LEGACY_REQUIRED_FIELDNAMES if schema_version == LEGACY_SCHEMA_VERSION else V2_REQUIRED_FIELDNAMES
    for field in required_fields:
        if field not in record or str(record[field]).strip() == "":
            errors.append(f"missing field: {field}")
    if errors:
        return errors
    if schema_version == LEGACY_SCHEMA_VERSION:
        style_profile = "iphone-realphoto"
        if record["style_signature"] != LEGACY_STYLE_SIGNATURE:
            errors.append(f"style_signature must be {LEGACY_STYLE_SIGNATURE}")
    elif schema_version == SCHEMA_VERSION:
        style_profile = str(record["style_profile"])
        if style_profile not in STYLE_PROFILES:
            errors.append("style_profile must be one of the six production art styles")
        else:
            if not style_signature_is_compatible(style_profile, str(record["style_signature"])):
                errors.append("style_signature must match style_profile")
            if str(record["style_family"]) != STYLE_PROFILES[style_profile]["family"]:
                errors.append("style_family must match style_profile")
        if str(record["subject_category"]) not in CATEGORY_CUES:
            errors.append("subject_category is not supported")
        if str(record["style_assignment"]) not in {STYLE_ASSIGNMENT, "explicit"}:
            errors.append(f"style_assignment must be {STYLE_ASSIGNMENT} or explicit")
        if "art_direction_signature" in record and str(record["art_direction_signature"]) != ART_DIRECTION_SIGNATURE:
            errors.append(f"art_direction_signature must be {ART_DIRECTION_SIGNATURE}")
        if "visual_archetype" in record and str(record["visual_archetype"]) not in VISUAL_ARCHETYPE_CUES:
            errors.append("visual_archetype is not supported")
        if "palette_profile" in record and str(record["palette_profile"]) not in PALETTE_PROFILES:
            errors.append("palette_profile is not supported")
    else:
        return [f"unsupported schema_version: {schema_version}"]
    try:
        index = int(str(record["index"]))
    except ValueError:
        errors.append("index must be an integer")
        index = -1
    if index in seen_indexes:
        errors.append(f"duplicate index: {index}")
    seen_indexes.add(index)
    if record["subject_kind"] not in {"physical-item", "tangible-token", "experience-scene"}:
        errors.append("subject_kind must be physical-item, tangible-token, or experience-scene")
    if "asset_width" in record and int(str(record["asset_width"])) != DEFAULT_ASSET_WIDTH:
        errors.append(f"asset_width must be {DEFAULT_ASSET_WIDTH}")
    if "asset_height" in record and int(str(record["asset_height"])) != DEFAULT_ASSET_HEIGHT:
        errors.append(f"asset_height must be {DEFAULT_ASSET_HEIGHT}")
    if "visual_grade" in record and str(record["visual_grade"]) not in GRADE_PALETTES:
        errors.append("visual_grade must be bronze, silver, gold, diamond, or legendary")
    if "frame_style" in record and "visual_grade" in record and str(record["frame_style"]) != FRAME_STYLES.get(str(record["visual_grade"])):
        errors.append("frame_style must match visual_grade")
    errors.extend(
        validate_prompt(
            str(record["prompt"]),
            style_profile,
            str(record.get("style_signature", "")).startswith(f"{STYLE_SET_SIGNATURE}/"),
            str(record.get("visual_archetype", "")) or None,
        )
    )
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
    schemas = sorted({str(record.get("schema_version", "")) for record in records})
    if schemas == [LEGACY_SCHEMA_VERSION]:
        return f"Manifest valid: {len(records)} records, style_signature={LEGACY_STYLE_SIGNATURE}"
    styles = sorted({str(record.get("style_profile", "iphone-realphoto")) for record in records})
    return f"Manifest valid: {len(records)} records, schemas={','.join(schemas)}, styles={','.join(styles)}"


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
                    "subject_category": record.get("subject_category"),
                    "style_profile": record.get("style_profile", "iphone-realphoto"),
                    "style_family": record.get("style_family", "iphone-realphoto"),
                    "style_assignment": record.get("style_assignment", "legacy"),
                    "art_direction_signature": record.get("art_direction_signature"),
                    "visual_archetype": record.get("visual_archetype"),
                    "palette_profile": record.get("palette_profile"),
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

    glow = Image.new("RGBA", base.size, (0, 0, 0, 0))
    glow_draw = ImageDraw.Draw(glow)
    glow_color = (*palette["glow"], 105)
    glow_draw.rounded_rectangle((18, 14, target_width - 18, target_height - 14), radius=30, outline=glow_color, width=14)
    glow = glow.filter(ImageFilter.GaussianBlur(10))
    overlay = Image.alpha_composite(overlay, glow)
    draw = ImageDraw.Draw(overlay)

    draw.rounded_rectangle((12, 10, target_width - 12, target_height - 10), radius=30, outline=(*palette["outer"], 255), width=18)
    draw.rounded_rectangle((32, 28, target_width - 32, target_height - 28), radius=22, outline=(*palette["mid"], 235), width=8)
    draw.rounded_rectangle((46, 42, target_width - 46, target_height - 42), radius=16, outline=(*palette["inner"], 185), width=3)

    corner = 84
    for sx, sy in ((1, 1), (-1, 1), (1, -1), (-1, -1)):
        x0 = 12 if sx > 0 else target_width - 12
        y0 = 10 if sy > 0 else target_height - 10
        plate = [
            (x0, y0),
            (x0 + sx * corner, y0),
            (x0 + sx * (corner - 24), y0 + sy * 24),
            (x0 + sx * 24, y0 + sy * (corner - 24)),
            (x0, y0 + sy * corner),
        ]
        draw.polygon(plate, fill=(*palette["outer"], 210), outline=(*palette["inner"], 210))

    rank = {"bronze": 1, "silver": 2, "gold": 3, "diamond": 4, "legendary": 5}[grade]
    start_x = target_width - 62 - (rank - 1) * 30
    y = target_height - 48
    for i in range(rank):
        draw_diamond(draw, start_x + i * 30, y, 10, (*palette["inner"], 230), (*palette["outer"], 255))

    if grade == "legendary":
        draw.rounded_rectangle((58, 52, target_width - 58, target_height - 52), radius=14, outline=(255, 70, 34, 100), width=3)
    if grade == "diamond":
        draw.rounded_rectangle((58, 52, target_width - 58, target_height - 52), radius=14, outline=(110, 240, 255, 110), width=3)

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


def render_review_sheets(
    images_dir: Path,
    review_out: Path,
    records: list[dict[str, object]],
    output_format: str,
    page_size: int = 20,
) -> list[str]:
    from PIL import Image, ImageDraw

    suffix = "." + output_format.lower()
    paths = sorted(images_dir.glob(f"*{suffix}"))
    if not paths:
        raise FileNotFoundError(f"no {suffix} images found in {images_dir}")
    review_out.mkdir(parents=True, exist_ok=True)
    for stale in review_out.glob("contact-*.png"):
        stale.unlink()

    record_by_index = {int(str(record["index"])): record for record in records if str(record.get("index", "")).isdigit()}
    columns = 4
    gap = 8
    margin = 12
    label_height = 22
    cell_width = FRONTEND_CARD_WIDTH
    cell_height = FRONTEND_CARD_HEIGHT + label_height
    written: list[str] = []

    for page_index, start in enumerate(range(0, len(paths), page_size), 1):
        page_paths = paths[start : start + page_size]
        rows = (len(page_paths) + columns - 1) // columns
        sheet_width = margin * 2 + columns * cell_width + (columns - 1) * gap
        sheet_height = margin * 2 + rows * cell_height + (rows - 1) * gap
        sheet = Image.new("RGB", (sheet_width, sheet_height), (14, 16, 20))
        draw = ImageDraw.Draw(sheet)
        for slot, path in enumerate(page_paths):
            row, column = divmod(slot, columns)
            x = margin + column * (cell_width + gap)
            y = margin + row * (cell_height + gap)
            with Image.open(path) as source:
                thumb = resize_cover(source, FRONTEND_CARD_WIDTH, FRONTEND_CARD_HEIGHT)
            sheet.paste(thumb, (x, y))
            try:
                index = int(path.stem)
            except ValueError:
                index = start + slot + 1
            record = record_by_index.get(index, {})
            style = str(record.get("style_profile", "unknown"))
            archetype = str(record.get("visual_archetype", "unknown"))
            draw.rectangle((x, y + FRONTEND_CARD_HEIGHT, x + cell_width, y + cell_height), fill=(20, 22, 28))
            draw.text((x + 6, y + FRONTEND_CARD_HEIGHT + 5), f"{index:04d}  {style}  {archetype}", fill=(232, 235, 240))
        output = review_out / f"contact-{page_index:04d}.png"
        sheet.save(output)
        written.append(str(output))
    return written


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
    if not args.dry_run and not args.no_render_review_sheets:
        review_out = Path(args.review_out) if args.review_out else out_dir / "review"
        for path in render_review_sheets(images_dir, review_out, records, args.output_format):
            print(f"review: {path}")
    print(f"manifest: {manifest_path}")
    print(f"image2_jobs: {jobs_path}")
    print(f"images: {images_dir}")
    if not args.dry_run and not args.no_render_card_frames:
        print(f"framed: {Path(args.frame_out) if args.frame_out else out_dir / 'framed'}")
    if not args.dry_run and not args.no_render_review_sheets:
        print(f"review: {Path(args.review_out) if args.review_out else out_dir / 'review'}")


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
    parser.add_argument("--list-styles", action="store_true", help="Print the six production art styles")
    parser.add_argument("--validate-manifest", help="Validate an existing JSONL or CSV prompt manifest")
    parser.add_argument("--manifest", help="Use an existing JSONL or CSV prompt manifest")
    parser.add_argument("--print-image-qa", action="store_true", help="Print the finished-image QA checklist")
    parser.add_argument("--check-image2-channel", action="store_true", help="Print image2 channel status without generating")
    parser.add_argument("--validate-images", help="Validate all images in a directory against --asset-size")
    parser.add_argument("--normalize-images", help="Resize/crop all images in a directory to --asset-size in place")
    parser.add_argument("--render-card-frames", help="Render deterministic game-card frame previews from an image directory")
    parser.add_argument("--frame-out", help="Output directory for framed card previews")
    parser.add_argument("--render-review-sheets", help="Render true-size 340x156 batch contact sheets from an image directory")
    parser.add_argument("--review-out", help="Output directory for batch contact sheets")
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
    parser.add_argument("--no-render-review-sheets", action="store_true")
    parser.add_argument("--visual-grade", choices=VISUAL_GRADES, default="bronze")
    parser.add_argument("--style-profile", choices=STYLE_CHOICES, default="auto")
    parser.add_argument("--style-seed", default=DEFAULT_STYLE_SEED, help="Stable style assignment seed")
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
    if args.list_styles:
        catalog = [
            {
                "style_profile": name,
                "style_family": STYLE_PROFILES[name]["family"],
                "style_signature": STYLE_PROFILES[name]["signature"],
            }
            for name in STYLE_PROFILE_ORDER
        ]
        print(json.dumps(catalog, ensure_ascii=False, indent=2))
        return
    if args.print_image_qa:
        checks = LEGACY_IMAGE_QA_CHECKS if args.style_profile == "iphone-realphoto" else IMAGE_QA_CHECKS
        print("\n".join(f"- {check}" for check in checks))
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
    if args.render_review_sheets:
        records = load_manifest(Path(args.manifest)) if args.manifest else []
        review_out = Path(args.review_out) if args.review_out else Path(args.render_review_sheets).parent / "review"
        for path in render_review_sheets(Path(args.render_review_sheets), review_out, records, args.output_format):
            print(f"review: {path}")
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
        records = [
            record_for(index, item, args.visual_grade, args.style_profile, args.style_seed)
            for index, item in enumerate(items, 1)
        ]
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
