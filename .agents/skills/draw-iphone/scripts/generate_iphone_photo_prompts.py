from __future__ import annotations

import argparse
import csv
import hashlib
import importlib.util
import json
import os
import re
import subprocess
import sys
from dataclasses import dataclass
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
    "item_id",
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
ITEM_ID_PATTERN = re.compile(r"[A-Za-z0-9][A-Za-z0-9._-]{0,127}")
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
    "办公室咖啡轮值": "plain takeaway coffee cup in a cardboard sleeve on an office pantry counter, lid texture and small condensation marks clearly visible, no coffee pot or thermal carafe",
    "泡面": "opened instant noodle cup on a kitchen counter, close enough to show noodles and broth surface",
    "打车票": "ride receipt on a car seat, private details blurred by shallow focus",
    "演唱会票": "concert ticket on a plain desk, no real artist name or readable barcode",
    "手机碎屏换新": "cracked smartphone on a real desk, sharp focus on fractured glass",
    "宠物急诊账单": "veterinary emergency deposit receipt on a clinic counter, private information not readable",
    "宠物急诊押金": "veterinary emergency deposit slip on a clinic counter, with one small abstract paw-print clinic mark and softly blurred treatment-room colors behind it, no readable private information",
    "酒店超售赔付": "one hotel overbooking compensation payment slip on a bright reception counter, with an abstract credit mark and a softly blurred room-key tray as restrained context, all printed details and personal information unreadable, no currency symbol or official emblem",
    "柜台小额手续费": "one small counter-service fee slip on a bright service desk, with a plain number token and payment tray as restrained context, all printed details abstract and unreadable, no currency symbol, government emblem, bank logo, or personal information",
    "跨城打车溢价": "one ride payment receipt representing a cross-city surge fare, placed diagonally on a bright car console with one abstract route line and blurred city lights as restrained context, no readable locations, amount, license plate, company logo, or personal information",
    "急诊留观押金": "one emergency observation deposit slip on a bright hospital admissions counter, with a plain patient wristband and softly blurred treatment curtain as restrained context, all printed details abstract and unreadable, no amount, hospital logo, official emblem, or personal information",
    "外卖退款到账": "one smartphone showing an abstract incoming refund confirmation for a food-delivery order, placed on a bright kitchen counter with one softly blurred delivery bag as restrained context, use only simple arrows and color blocks with no readable interface text, amount, app logo, or personal information",
    "儿童安全座椅": "one modern child safety car seat as the only product hero, shown in a dynamic three-quarter view with the complete shell, headrest, side-impact wings, five-point harness, buckle, padding, and base clearly visible, bright commercial rim light, no child, adult, vehicle cabin, packaging, logo, certification mark, or readable label",
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
    "幼儿园手工作业材料": "one complete children's craft-material kit as the only product hero, with colored paper, blunt safety scissors, glue stick, soft pom-poms, and wooden craft sticks arranged as one compact usable set, every component fully visible, no finished school project, child, hand, readable packaging, school logo, or duplicate kit",
    "专业教材整套": "one complete coordinated set of professional textbooks as the only product hero, five sturdy volumes arranged in a dynamic stepped stack with abstract unreadable cover diagrams and visible paper texture, no real title, author, publisher logo, loose notebook, stationery, hand, person, or duplicate book pile",
    "笔记本屏幕总成": "one complete replacement laptop display assembly as the only product hero, shown from a dynamic three-quarter rear angle with intact thin panel, bezel, hinge mounts, display cable, and metal backing clearly visible, no laptop base, broken glass, hand, repair bench, brand, serial label, readable marking, packaging, or duplicate screen",
    "台式机显卡烧毁": "one desktop graphics card with localized heat damage as the only product hero, shown in a dynamic three-quarter view with complete circuit board, cooling shroud, fans, connector edge, and one restrained scorched power area clearly visible, no flames, smoke cloud, computer case, hand, repair shop, brand, readable label, or duplicate card",
    "深夜牛肉面": "one steaming bowl of late-night beef noodles as the only food hero, with clear noodles, sliced beef, broth surface, and restrained green garnish visible from a low three-quarter angle, no diner, hand, restaurant sign, menu, brand, readable bowl mark, extra side dish, or duplicate bowl",
    "大杯手冲咖啡": "one large freshly brewed pour-over coffee in a clear server and plain cup as one coherent drink set, shown from a dynamic three-quarter angle with warm translucent coffee and restrained steam, no barista, hand, cafe sign, brand, readable marking, pastry, duplicate cup, or retail packaging",
}

SERVICE_SCENE_DETAILS = {
    "钱柜KTV（包厢预订）": "one vivid private-room KTV experience centered on a glowing karaoke display and a single microphone in a premium empty room ready for guests, no booking document or readable lyrics",
    "ChatGPT Plus 订阅": "one modern creator workspace centered on a laptop displaying an abstract AI conversation interface, one seated user silhouette optional, no confirmation email, voucher, receipt, or readable screen text",
    "Keep健身App年卡": "one energetic fitness-app training moment centered on a single athlete running on a treadmill with one nearby phone showing abstract workout rings, no membership card or readable interface text",
    "携程租车（自驾游SUV）": "one modern SUV as the clear hero driving through a broad scenic mountain road, communicating a self-drive rental journey without any reservation document or readable road sign",
    "盒马鲜生（生鲜配送）": "one fresh-grocery delivery moment centered on a chilled delivery tote at a bright apartment doorway, seafood and produce limited to supporting context, no order receipt or readable branding",
    "叮咚买菜": "one doorstep grocery-delivery moment centered on a courier handing over a single vegetable-filled delivery bag, limited supporting produce, no order receipt or readable branding",
    "摄影工作室（婚纱照套系）": "one cinematic wedding portrait session in a professional studio, a bride and groom forming one focal pair under softbox light with a camera silhouette in the foreground, no booking voucher or readable text",
    "冰箱急修上门": "one home appliance technician actively repairing an open refrigerator in an ordinary kitchen, centered on the technician, exposed rear service panel, and compact tool case as one coherent repair action, no sales showroom or standalone product advertisement",
    "充电宝超时分钟费": "one close dynamic action of returning a shared power bank into a glowing rental kiosk, with the power bank and return slot forming one clear focal cluster slightly right of center, a bright abstract overdue timer ring without readable digits, no phone payment page or receipt",
    "奶茶全员请客": "one office employee setting down the final drink carrier beside a neat group order of colorful milk-tea cups on a bright pantry table, the carrier and cups forming one abundant focal cluster slightly right of center, no crowd, no brand marks, and no readable cup labels",
    "演唱会前排连锁": "one exhilarating front-row concert moment centered on a luminous stage, barricade, and one excited audience silhouette slightly right of center, brilliant moving beams and sparkling highlights, no performer likeness, no readable screens, ticket, hotel document, or collage",
    "地面找平补差": "one renovation worker pouring self-leveling compound across an apartment floor while guiding it with a gauge rake and laser level, the glossy leveling wave and tool action slightly right of center, bright work lights, no quote sheet, invoice, or finished-room glamour shot",
    "暑期游泳班": "one indoor swimming lesson with an instructor at the pool edge helping one child practice a kickboard drill in a single clear lane, the teaching action slightly right of center, radiant blue water reflections and bright summer energy, no crowd, signage, or readable lane text",
    "资格证培训班": "one focused adult certification-training session in a compact bright classroom, centered slightly right on one learner working through practice material while an instructor points to abstract diagrams on a board, no readable questions, logos, certificates, or crowded lecture hall",
    "外地面试差旅": "one business candidate arriving in another city for an interview, walking through a bright rail-station concourse with one rolling suitcase and a plain document folder, the determined traveler slightly right of center, no readable signs, tickets, company logos, or split travel montage",
    "签证材料加急": "one urgent visa-document preparation service at a bright agency counter, centered slightly right on a staff member rapidly organizing one passport-sized booklet and a tidy application packet under a desk lamp, no readable forms, flags, official seals, or government emblems",
    "摄影加机位": "one cinematic wedding shoot where two professional photographers capture the same bride-and-groom focal pair from complementary angles, cameras and softbox lights forming one coherent action slightly right of center, bright celebratory rim light, no venue signage or booking document",
    "私人会所年费": "one exclusive private-club arrival in a luminous contemporary lounge, centered slightly right on a host opening the inner door for one smartly dressed guest, polished stone and warm metal highlights communicating premium access, no membership card, receipt, logo, or crowded party",
    "变速箱深度保养": "one skilled mechanic performing deep transmission maintenance beneath a raised car, with the exposed gearbox housing, fluid service equipment, and mechanic forming one precise focal cluster slightly right of center, brilliant workshop task lights, no invoice, branding, or generic car showroom",
    "工作餐升级套餐": "one office canteen worker sliding an upgraded lunch tray toward one employee, with one main dish and one clearly added side dish forming a bright appetizing focal cluster slightly right of center, no menu board, receipt, crowd, brand mark, or readable packaging",
    "同城急送跑腿": "one same-city courier making an urgent parcel handoff at a bright building entrance, with the compact parcel and courier forming one dynamic focal cluster slightly right of center, a parked delivery scooter only as supporting context, no phone order screen, receipt, logo, address, or readable label",
    "车险免赔差额": "one car-insurance inspection in a bright repair bay, centered slightly right on an adjuster and vehicle owner examining one visible damaged body panel that insurance does not fully cover, no estimate sheet, receipt, company logo, license plate, crash scene, or split panel",
    "家庭临时请客": "one lively last-minute family banquet in a bright private dining room, centered slightly right on a host setting down one abundant shared dish for two seated relatives as one coherent focal group, no crowd, menu, restaurant logo, receipt, or readable table sign",
    "滞纳金叠加": "one administrative service-counter moment where a clerk places the latest fee notice onto a small visibly growing stack while one customer reacts, forming one clear focal action slightly right of center under bright task lighting, all document marks abstract and unreadable, no amount, logo, official emblem, or collage",
    "高级清洁费": "one premium venue-cleaning service after an event, centered slightly right on one professional cleaner polishing a luminous marble floor beside a compact cleaning machine, sparkling reflections and warm architectural highlights, no party crowd, invoice, hotel logo, or luxury product display",
    "部门庆功请客": "one office team celebration centered slightly right on a manager setting down a bright celebration cake and a small shared meal for three colleagues, one coherent festive focal group with radiant commercial light, no crowd, banner text, company logo, receipt, or alcohol branding",
    "合同律师审核": "one focused contract-review meeting in a bright modern law office, centered slightly right on one lawyer pointing to a single open agreement while one client listens, with abstract unreadable lines and a plain folder, no courtroom, official seal, law-firm logo, signature, or readable clause",
    "便利店早餐十连": "one convenience-store breakfast rush where a cashier places the latest warm breakfast bundle beside several neatly grouped buns, drinks, and wrapped staples on a bright counter, forming one abundant focal cluster slightly right of center, no brand, receipt, retail pricing display, readable packaging, or crowd",
    "超市一周补货": "one weekly grocery restock moment centered slightly right on a shopper loading a full but orderly cart with fresh produce, household staples, and one reusable bag in a bright supermarket aisle, no brand, price sign, receipt, crowded aisle, or readable packaging",
    "慈善晚宴座位": "one elegant charity-gala arrival centered slightly right on a host guiding one guest to a single reserved place at a luminous formal dining table, with polished glassware and warm stage light as restrained context, no crowd, donation check, name card, banner, logo, or readable table sign",
    "签证加急费": "one urgent visa-processing service centered slightly right on an agency specialist sealing one complete passport-sized booklet and application packet into a secure express pouch under bright task lighting, no readable form, flag, official seal, government emblem, receipt, or courier logo",
    "周末搬家加价": "one weekend moving service centered slightly right on two movers carrying a sofa through a bright apartment doorway toward a waiting unbranded moving truck, one coherent high-effort action with limited boxes as context, no invoice, price label, company logo, address, crowd, or split panel",
    "沙发定金": "one furniture-showroom purchase moment centered slightly right on a customer testing one modern sofa while a sales consultant presents fabric swatches nearby, the complete sofa remains the dominant object under bright commercial light, no payment receipt, retail pricing display, brand, readable swatch label, or crowded showroom",
    "七座车异地还车": "one out-of-town rental return centered slightly right on a traveler handing a plain car key to an attendant beside a clean seven-seat vehicle at a bright return bay, luggage only as restrained context, no rental document, logo, license plate, fuel price, or readable sign",
    "研究生复试辅导": "one graduate interview-coaching session in a bright classroom, centered slightly right on one mentor conducting a mock panel interview with one student seated opposite, abstract notes only, no university logo, certificate, readable questions, crowd, or lecture-hall scene",
    "胃镜麻醉套餐": "one calm pre-procedure consultation in a bright endoscopy preparation room, centered slightly right on a clinician explaining sedation preparation to one seated adult beside clean monitoring equipment, non-graphic and reassuring, no procedure in progress, exposed body, medical advice text, hospital logo, form, or readable screen",
    "厨房水电增项": "one kitchen-renovation worker installing newly added plumbing and electrical conduits inside an unfinished cabinet wall, the pipes, cable runs, and skilled action forming one clear focal cluster slightly right of center under bright work lights, no quote sheet, invoice, brand, unsafe exposed live wire, or finished luxury kitchen",
    "二手设备卖出": "one second-hand equipment sale centered slightly right on a seller handing one clean used laptop to a buyer at a bright resale counter, the device and handoff forming one coherent incoming-money moment, no cash, payment screen, marketplace logo, serial number, receipt, or readable device interface",
    "商标注册服务费": "one trademark-registration consultation in a bright intellectual-property service office, centered slightly right on one adviser comparing a single abstract symbol sketch with one client beside a plain application folder, no readable text, real trademark, official seal, government emblem, receipt, law-firm logo, or crowded office",
    "考前冲刺课程": "one focused exam-cram lesson in a compact bright classroom, centered slightly right on one tutor guiding one student through a timed practice problem on a tablet beside abstract diagrams, no readable question, answer, score, school logo, certificate, crowd, or lecture-hall scene",
    "计算机等级报名": "one computer-exam registration moment at a bright campus service desk, centered slightly right on one student presenting a plain identity-sized card to an attendant beside a desktop monitor with abstract color blocks, no readable interface, personal data, exam logo, school emblem, receipt, or queue",
    "共享车押金找回失败": "one failed shared-bicycle deposit-recovery moment at a bright bicycle docking area, centered slightly right on one frustrated rider checking a phone with an abstract warning symbol beside a correctly parked unbranded bicycle, no readable interface, amount, app logo, receipt, cash, crowd, or damaged bicycle",
    "节假日租车三天": "one cheerful three-day holiday car-rental pickup at a bright rental bay, centered slightly right on one traveler receiving a plain car key beside a clean compact crossover with a small suitcase as restrained context, no rental document, logo, license plate, fuel price, readable sign, crowd, or road-trip montage",
    "押金原路退回": "one successful deposit-return moment at a bright service counter, centered slightly right on one customer receiving a plain bank card and a returned key token while a small green confirmation light glows on the payment terminal, no readable interface, amount, cash, receipt, bank logo, personal data, or crowd",
    "停车超时补费": "one parking-overtime payment moment at a bright garage exit kiosk, centered slightly right on one driver tapping a plain bank card while the barrier waits beside the car, no readable timer, amount, receipt, payment logo, license plate, attendant, traffic queue, or dark threatening atmosphere",
    "合同违约金": "one contract-penalty settlement meeting in a bright modern office, centered slightly right on two parties reviewing one open agreement while a mediator points to a single abstract highlighted clause, no readable text, signature, cash, official seal, company logo, courtroom, crowd, or aggressive confrontation",
    "四条轮胎更换": "one complete four-tire replacement service in a brilliant modern workshop, centered slightly right on one mechanic fitting a new tire to a raised everyday car while the other three matching tires form a neat supporting stack, no invoice, tire brand, workshop logo, license plate, crowd, or damaged crash scene",
    "夜路爆胎救援": "one urgent but controlled nighttime roadside tire rescue, centered slightly right on one safety-vest mechanic changing a visibly flat tire under brilliant portable work lights beside an everyday car, reflective cones as limited context, no injury, crash, rain, tow-company logo, license plate, readable sign, or horror atmosphere",
    "空调漏水返修": "one air-conditioner leak repair in a bright lived-in apartment, centered slightly right on one technician opening the wall unit and clearing a blocked drain line above a small protected floor area, a few water droplets visible as evidence, no invoice, brand, mold, electrical hazard, flooded room, crowd, or finished-product advertisement",
    "核磁共振加项": "one calm MRI add-on preparation in a bright modern imaging room, centered slightly right on one clinician reassuring one seated adult beside the recognizable MRI scanner, non-graphic and dignified, no procedure in progress, exposed body, diagnosis, medical advice text, readable screen, hospital logo, form, or crowd",
    "长辈寿宴礼金": "one respectful elder birthday banquet in a luminous private dining room, centered slightly right on one younger relative presenting a single plain red envelope to the seated elder beside a restrained celebration table, no readable decoration, age number, cash, crowd, restaurant logo, receipt, or wedding imagery",
    "婚纱礼服尾款": "one wedding-dress final fitting in a brilliant bridal atelier, centered slightly right on one bride viewing the complete gown in a full-length mirror while one tailor adjusts the hem, no payment counter, receipt, brand, readable signage, groom, crowd, or photo-shoot montage",
    "停车起步补费": "one small parking-exit payment moment in a bright garage, centered slightly right on a driver tapping a plain card at the exit kiosk while the barrier begins to rise, no readable amount, timer, receipt, logo, license plate, queue, or dark atmosphere",
    "设计图修改费": "one interior-design revision meeting in a bright studio, centered slightly right on one designer moving a wall line on an abstract floor plan while one homeowner reviews a material sample, no readable dimensions, invoice, company logo, official stamp, crowded desk, or finished-room montage",
    "朋友突然还钱": "one warm repayment moment between two friends at a bright cafe table, centered slightly right on one friend returning a plain bank card and small borrowed key token while the other reacts with relief, no cash, readable phone screen, amount, receipt, brand, crowd, or gift-giving scene",
    "同事离职礼物": "one sincere office farewell gift moment, centered slightly right on one colleague handing a single neatly wrapped practical gift to a departing coworker beside a clean desk, no crowd, party banner, readable card, brand, cash, alcohol, or birthday imagery",
    "前挡玻璃修复": "one precise windshield chip repair in a brilliant automotive workshop, centered slightly right on one technician using a resin bridge tool over a small visible chip on an intact windshield, no shattered glass, crash, invoice, brand, license plate, crowd, or full replacement scene",
    "房产过户测绘费": "one property-transfer surveying service in a bright empty apartment, centered slightly right on one surveyor measuring the room with a laser tripod while the owner observes, no deed, readable dimensions, government emblem, receipt, real-estate logo, crowd, or luxury staging",
    "儿童乐园临时票": "one lively indoor children's playground visit, centered slightly right on one child entering a colorful climbing area while a guardian waits at the gate, bright safe equipment and joyful motion, no ticket document, readable wristband, brand, crowd, mascot, or unsafe play",
    "证件照加洗": "one extra identity-photo printing service in a bright portrait studio, centered slightly right on one technician collecting several small freshly printed portrait sheets from a compact photo printer, all faces abstract and non-identifying, no readable personal data, official emblem, receipt, brand, or crowd",
    "全屋深度保洁": "one whole-home deep-cleaning service in a bright lived-in apartment, centered slightly right on two professional cleaners working as one coherent team on glass and floor surfaces, sparkling controlled highlights, no crowd, invoice, brand, hotel lobby, unsafe chemical cloud, or before-and-after split panel",
    "生日聚餐请客": "one cheerful birthday dinner treat in a bright private dining room, centered slightly right on the host presenting one small cake beside a shared meal for two friends, no age number, readable banner, crowd, restaurant logo, receipt, cash, or wedding decoration",
    "营业执照代办": "one business-registration agency consultation in a bright administrative office, centered slightly right on one adviser organizing a plain application folder for a small-business owner, no readable license, official seal, government emblem, receipt, company logo, queue, or courtroom",
    "轮胎补气救援": "one quick roadside tire-inflation rescue in clear daylight, centered slightly right on one mechanic connecting a portable compressor to a visibly low tire beside an everyday car, no flat-tire replacement, crash, invoice, company logo, license plate, traffic queue, or night scene",
    "优惠券过期反买": "one expired-coupon checkout setback in a bright shop, centered slightly right on one shopper reconsidering a purchase while a cashier points to an abstract warning symbol on the payment terminal, no readable interface, amount, coupon code, brand, receipt, crowd, or duplicate product pile",
    "公证材料翻译": "one notarized-document translation service in a bright language office, centered slightly right on one translator comparing two neatly aligned abstract document pages for a client, no readable language, personal data, official seal, government emblem, receipt, flag, or crowd",
    "机场高速过路费": "one airport-expressway toll payment in brilliant daylight, centered slightly right on a traveler tapping a plain card at a modern toll gate from an everyday car, aircraft silhouette only as distant context, no readable amount, road sign, receipt, toll logo, license plate, or traffic queue",
    "机场接送包车": "one private airport-transfer pickup outside a bright terminal, centered slightly right on a professional driver loading one suitcase into a clean unbranded people carrier for one traveler, no booking document, readable terminal sign, company logo, license plate, crowd, or travel montage",
    "办公室咖啡六杯": "one office coffee run centered slightly right on an employee setting down a complete six-cup drink carrier on a bright pantry counter, all six cups clearly visible as one abundant cluster, no crowd, spilled drink, readable cup marks, cafe logo, receipt, or extra food",
    "网约车取消费": "one ride-cancellation fee moment at a bright curbside pickup point, centered slightly right on one waiting passenger checking a phone with a simple abstract cancellation symbol as the requested car departs in the distance, no readable interface, amount, app logo, receipt, license plate, crowd, or angry confrontation",
    "牙科种植定金": "one calm dental-implant planning consultation in a bright modern clinic, centered slightly right on one dentist explaining a simple tooth model to one seated adult, non-graphic and reassuring, no procedure in progress, exposed mouth close-up, diagnosis text, deposit receipt, hospital logo, or crowd",
    "跨境认证服务费": "one cross-border certification service in a bright international documentation office, centered slightly right on one specialist placing a plain application packet into a secure document sleeve for a client, no readable language, flag, official seal, government emblem, receipt, courier logo, or crowd",
    "电脑主板维修": "one expert laptop motherboard repair in a brilliant electronics workshop, centered slightly right on one technician using a precision soldering tool and magnifier over the exposed circuit board, no sparks, damaged battery, brand, serial number, invoice, readable screen, or cluttered device pile",
    "定制柜增项": "one custom-cabinet addition being installed in a bright apartment, centered slightly right on one carpenter fitting an added cabinet module into a measured wall opening, material panels and level as limited context, no quote sheet, invoice, brand, unsafe tool use, crowd, or finished-room glamour shot",
    "仓储保管费": "one orderly warehouse-storage service centered slightly right on one worker placing a sealed household box onto a clean numbered rack with a small trolley, no readable label, address, invoice, warehouse logo, crowd, dangerous stacking, or abandoned dirty warehouse",
    "游戏账号申诉费": "one game-account appeal support session in a bright digital-service office, centered slightly right on one support specialist helping one customer review an abstract account-recovery screen, no readable interface, game artwork, username, password, payment receipt, platform logo, crowd, or gaming montage",
    "商务形象顾问": "one professional image-consulting session in a bright wardrobe studio, centered slightly right on one consultant adjusting a client's jacket fit while comparing two restrained fabric swatches, no fashion brand, mirror text, crowd, runway, shopping receipt, or makeover split panel",
    "古玩鉴定服务费": "one antique-appraisal service in a bright specialist studio, centered slightly right on one appraiser examining a single small ceramic object with a loupe while the owner observes, no readable certificate, auction logo, receipt, cash, crowded collection, museum label, or dramatic treasure glow",
    "航班延误赔付": "one flight-delay compensation moment at a bright airline service counter, centered slightly right on one traveler receiving a plain bank card and travel pouch from an attendant, no readable flight number, amount, receipt, airline logo, passport data, crowd, or aircraft accident imagery",
    "少儿体检套餐": "one gentle child health-check consultation in a bright pediatric clinic, centered slightly right on one clinician measuring a cooperative child's height while one parent watches, non-graphic and reassuring, no diagnosis, injection, exposed body, readable chart, hospital logo, crowd, or medical form",
    "走错车道罚单": "one wrong-lane traffic enforcement moment in clear daylight, centered slightly right on one driver speaking calmly with a traffic officer beside a safe roadside lane divider, no readable ticket, cash, official emblem, police logo, license plate, crash, confrontation, or traffic jam",
    "卫生间防水重做": "one bathroom waterproofing rework in a bright unfinished apartment, centered slightly right on one renovation worker applying a continuous waterproof membrane across the floor and lower walls, roller and sealed drain as limited context, no invoice, brand, standing water, exposed live wire, crowd, or finished luxury bathroom",
    "退税突然到账": "one successful tax-refund arrival at a bright service desk, centered slightly right on one relieved customer receiving a plain bank card from a tax adviser beside a small green confirmation light, no readable amount, tax form, official seal, government emblem, cash, receipt, logo, or crowd",
    "家电套装尾款": "one home-appliance suite final-payment moment in a brilliant showroom, centered slightly right on a customer confirming a coordinated refrigerator, washer, and compact oven set with one sales consultant, all three appliances readable as one coherent package, no payment screen, receipt, retail pricing display, brand, crowd, or duplicate appliance row",
    "行李延误临时采购": "one delayed-luggage emergency shopping moment in a bright airport store, centered slightly right on one traveler placing a compact set of basic clothing and toiletries into a plain carry bag, an empty baggage carousel only as distant context, no readable airport sign, airline logo, receipt, brand, crowd, or lost suitcase montage",
    "装修返工增项": "one renovation rework scene in a bright unfinished apartment, centered slightly right on two workers removing one incorrectly installed wall panel and preparing the corrected surface as one coherent action, no invoice, quote sheet, brand, unsafe demolition, crowd, finished-room glamour shot, or before-and-after split panel",
    "小奖到账": "one modest prize payout moment at a bright neighborhood lottery counter, centered slightly right on one smiling clerk returning a plain bank card to a pleasantly surprised customer beside a small celebratory light, no readable ticket, number, amount, cash, lottery logo, crowd, or jackpot spectacle",
    "保险理赔到账": "one successful insurance-claim payout at a bright service office, centered slightly right on one claim adviser returning a plain bank card to a relieved customer beside a closed damage-photo folder, no readable policy, amount, cash, receipt, insurer logo, accident scene, crowd, or official seal",
    "周末自助早餐": "one inviting weekend breakfast buffet in a luminous hotel dining room, centered slightly right on one guest selecting a warm breakfast plate from a compact buffet with bread, eggs, fruit, and coffee as one coherent spread, no crowd, menu, hotel logo, retail display, receipt, or excessive food pile",
    "双人奶茶外送": "one two-person milk-tea delivery handoff at a bright apartment doorway, centered slightly right on a courier presenting one secure two-cup carrier to a customer, both drinks fully visible with convincing condensation, no crowd, spilled drink, readable cup marks, delivery logo, receipt, or extra food",
    "商务茶歇套餐": "one polished business tea-break service in a bright meeting lounge, centered slightly right on an attendant arranging one coherent tray of tea, coffee, and small pastries for two conference guests, no crowd, menu, hotel logo, readable cup marks, receipt, alcohol, or banquet scene",
    "聚会香槟套餐": "one elegant small-party champagne service in a luminous private lounge, centered slightly right on a host pouring from one unbranded bottle into two sparkling flutes beside restrained appetizers, no crowd, readable label, alcohol brand, receipt, nightclub darkness, excessive bottles, or wedding toast",
    "高峰打车加价": "one peak-hour ride pickup in a bright busy city, centered slightly right on one passenger entering an unbranded car while dense but orderly traffic glows behind, urgency communicated through movement and traffic rather than a payment screen, no readable app, amount, receipt, logo, license plate, crash, or crowd",
    "跨城顺风车补差": "one cross-city rideshare pickup at a bright highway service area, centered slightly right on a traveler placing one suitcase into an everyday driver's open trunk before departure, no payment screen, amount, receipt, rideshare logo, license plate, crowd, or road-trip montage",
    "软件专业版续费": "one professional-software renewal decision at a bright creative workstation, centered slightly right on one designer confirming an abstract feature upgrade with a support specialist beside the monitor, no readable interface, software logo, payment screen, receipt, account data, crowd, or futuristic hologram",
    "周末绘画体验课": "one joyful weekend children's painting lesson in a bright art classroom, centered slightly right on one instructor guiding one child painting broad abstract color shapes on a small canvas, no crowd, readable artwork text, school logo, certificate, payment receipt, messy wall graffiti, or famous painting copy",
    "牙齿矫正定金": "one calm child orthodontic planning consultation in a bright dental clinic, centered slightly right on one orthodontist showing a simple brace model to a child and parent, non-graphic and reassuring, no procedure in progress, exposed-mouth close-up, diagnosis text, deposit receipt, clinic logo, or crowd",
    "私立学校校车费": "one private-school bus boarding moment in clear morning light, centered slightly right on one child stepping safely onto a clean unbranded school bus while a guardian and driver supervise, no crowd, readable school name, uniform logo, fee document, receipt, traffic danger, or luxury limousine",
    "海外研学团费": "one overseas study-tour departure in a bright airport concourse, centered slightly right on one student with a backpack and small suitcase receiving a plain travel folder from a teacher while a parent says goodbye, no crowd, readable destination, flag, school logo, payment document, passport data, or travel montage",
    "校园打印装订": "one campus printing-and-binding service in a bright copy shop, centered slightly right on one operator feeding a neat academic paper stack into a binding machine for one student, no readable paper text, thesis title, school logo, receipt, crowd, loose document mess, or office portrait printing",
    "语言考试报名费": "one language-exam registration moment at a bright education service desk, centered slightly right on one student presenting a plain identity-sized card while an attendant confirms abstract fields on a monitor, no readable interface, exam logo, personal data, receipt, official seal, queue, or classroom test scene",
    "异地考试住宿": "one out-of-town exam hotel arrival in a bright modest lobby, centered slightly right on one student checking in with a small suitcase and closed study folder, no readable hotel sign, room price, booking document, school logo, crowd, luxury resort, or travel montage",
    "面试西装租赁": "one interview-suit rental fitting in a bright menswear studio, centered slightly right on one consultant adjusting a complete well-fitted suit for a job candidate beside one garment rack, no fashion brand, receipt, crowd, runway, wedding styling, or duplicate outfit display",
    "职业证件照套餐": "one professional headshot session in a bright portrait studio, centered slightly right on one job candidate seated against a plain backdrop while one photographer adjusts a softbox and camera, no printed portrait sheet, readable resume, company logo, crowd, fashion shoot, or passport-office scene",
    "行业峰会门票": "one industry-summit arrival in a brilliant conference venue, centered slightly right on one professional entering a focused keynote hall while an attendant scans a plain abstract pass, no readable event name, badge text, company logo, crowd, ticket close-up, receipt, or trade-show booth collage",
    "海外会议参会费": "one overseas business-conference participation moment in a luminous international meeting hall, centered slightly right on one professional presenting to a small seated panel with an abstract chart behind, no readable language, flag, company logo, crowd, airline imagery, receipt, or conference montage",
    "住院检查预缴": "one hospital examination prepayment consultation at a bright admissions desk, centered slightly right on one patient speaking with a clerk while a clinician holds a plain examination folder in the background, no readable form, amount, cash, receipt, hospital logo, diagnosis, exposed body, or crowd",
    "宠物住院押金": "one pet-hospital admission consultation in a bright veterinary clinic, centered slightly right on one veterinarian reassuring an owner beside a calm small dog resting in a clean carrier, no procedure, injury, exposed body, deposit receipt, clinic logo, readable form, cage row, or crowd",
    "宠物心脏手术定金": "one serious but reassuring veterinary heart-surgery planning consultation in a bright clinic, centered slightly right on one veterinarian explaining an abstract heart scan to an owner beside a calm small dog, no operation in progress, injury, exposed body, diagnosis text, deposit receipt, clinic logo, or crowd",
    "返程机票改签": "one return-flight rebooking service at a bright airport counter, centered slightly right on one traveler handing a plain travel folder to an attendant who adjusts an abstract itinerary on the monitor, no readable flight number, destination, airline logo, ticket close-up, receipt, passport data, crowd, or cancelled-flight chaos",
    "海外租车押金": "one overseas rental-car deposit moment in a bright vehicle pickup bay, centered slightly right on one traveler tapping a plain bank card while an attendant presents the key beside a clean compact car, no readable country sign, flag, amount, receipt, rental logo, license plate, crowd, or road-trip montage",
    "全屋美缝追加": "one whole-home grout-finishing addition in a bright tiled apartment, centered slightly right on one skilled worker applying clean contrasting grout along floor and wall joints with a precision tool, no invoice, quote sheet, brand, crowd, finished luxury staging, dirty demolition, or before-and-after split panel",
    "拍卖图录服务费": "one auction-catalog preparation service in a bright preview room, centered slightly right on one specialist photographing a single collectible object while a client reviews an abstract catalog layout, no readable lot number, auction logo, receipt, cash, crowded collection, bidding scene, or famous artwork",
    "违停拖车保管费": "one vehicle-retrieval moment at a bright municipal storage yard, centered slightly right on one driver receiving a plain car key from an attendant beside the safely parked towed car, tow truck only as restrained background context, no readable ticket, amount, official emblem, cash, receipt, license plate, crowd, or confrontation",
    "电瓶道路救援": "one controlled roadside battery rescue in clear daylight, centered slightly right on one mechanic connecting a portable jump starter under the open hood of an everyday car while the driver waits safely, no crash, flames, tow-company logo, invoice, license plate, traffic queue, night scene, or tire repair",
    "话费重复扣款退回": "one duplicate mobile-charge refund at a bright telecom service counter, centered slightly right on one adviser returning a plain bank card to a relieved customer beside a phone with a simple abstract refund arrow, no readable interface, amount, phone number, telecom logo, receipt, cash, crowd, or device sale",
    "租房押金退回": "one rental-deposit return after a successful apartment move-out inspection, centered slightly right on one landlord returning a plain bank card while the tenant hands back a single apartment key in a clean empty room, no readable contract, amount, cash, receipt, property logo, moving boxes, crowd, or dispute",
    "年终奖补发": "one delayed year-end bonus correction in a bright payroll office, centered slightly right on one payroll specialist returning a plain bank card to a pleasantly surprised employee beside an abstract confirmation light, no cash, red envelope, readable payslip, amount, company logo, receipt, crowd, or award ceremony",
    "伴郎礼服押金": "one best-man suit rental fitting in a bright formalwear studio, centered slightly right on one attendant adjusting a complete suit for the best man while a plain garment bag waits nearby, no payment receipt, amount, brand, crowd, wedding ceremony, runway, or duplicate outfit display",
    "救护车费用": "one controlled ambulance transport arrival at a bright hospital entrance, centered slightly right on two paramedics safely guiding one seated patient from an unbranded ambulance toward care, non-graphic and calm, no injury, diagnosis, readable hospital sign, invoice, amount, emergency logo, crowd, or crash scene",
    "住院押金": "one hospital admission moment at a bright inpatient desk, centered slightly right on one patient speaking with a clerk while a nurse prepares a clean room wristband, no readable form, amount, cash, receipt, hospital logo, diagnosis, exposed body, or crowd",
    "机票改签费": "one flight-change service at a bright airport counter, centered slightly right on one traveler handing a plain travel folder to an attendant who updates an abstract itinerary on a monitor, no readable flight number, destination, airline logo, ticket close-up, receipt, amount, passport data, crowd, or cancelled-flight chaos",
    "欧洲自驾押金": "one European self-drive rental deposit moment in a bright scenic vehicle pickup bay, centered slightly right on one traveler tapping a plain bank card while an attendant presents a car key beside a clean touring car, no flag, readable country sign, amount, receipt, rental logo, license plate, crowd, or road-trip montage",
    "宴会酒水押金": "one banquet beverage deposit service in a luminous event preparation room, centered slightly right on one catering manager confirming a restrained set of unbranded bottles and glassware with the host, no payment receipt, amount, readable label, alcohol brand, crowd, active party, or excessive bottle wall",
    "私立医院住院押金": "one private-hospital admission consultation in a luminous patient suite, centered slightly right on one admissions specialist guiding a patient and family member through room preparation, no readable form, amount, cash, receipt, hospital logo, diagnosis, exposed body, crowd, or luxury hotel imagery",
    "行李超重补费": "one overweight-baggage resolution at a bright airport check-in counter, centered slightly right on one traveler moving an item from an open suitcase into a carry bag while an attendant watches the baggage scale, no readable weight, amount, airline logo, receipt, passport data, crowd, or luggage-loss scene",
    "全家返程机票": "one family return-flight departure in a bright airport concourse, centered slightly right on two parents and one child moving together with a compact set of luggage toward the gate, no ticket close-up, readable destination, flight number, airline logo, passport data, crowd, or travel montage",
    "婚车超时费用": "one wedding-car overtime handoff outside a bright reception venue, centered slightly right on a driver waiting beside a decorated but unbranded car while the newly married couple hurries from the entrance, no readable clock, amount, invoice, license plate, crowd, traffic jam, or wedding montage",
    "超跑赛道事故押金": "one supercar track-damage inspection in a brilliant pit lane, centered slightly right on one track engineer and driver examining a single visibly scraped body panel on a parked sports car, no crash in progress, injury, flames, amount, deposit receipt, brand, license plate, crowd, or racing montage",
    "油画保税仓费用": "one bonded-art-storage service in a bright climate-controlled warehouse, centered slightly right on two gloved specialists placing a single protected framed painting onto a secure rack, no visible famous artwork, readable inventory label, customs emblem, invoice, amount, crowd, or dirty warehouse",
    "车辆保险赔付": "one successful vehicle-insurance payout at a bright repair office, centered slightly right on one claim adviser returning a plain bank card to a relieved driver beside one repaired everyday car, no readable policy, amount, cash, receipt, insurer logo, crash scene, license plate, or crowd",
    "酒店取消退款": "one successful hotel-cancellation refund at a bright reception desk, centered slightly right on one receptionist returning a plain bank card to a relieved traveler beside a returned room-key sleeve, no readable booking, amount, cash, receipt, hotel logo, crowd, or argument",
    "房屋退租押金到账": "one move-out deposit return after a successful home inspection, centered slightly right on one property manager returning a plain bank card while the former tenant hands back a single key in a clean empty apartment, no readable lease, amount, cash, receipt, property logo, moving boxes, crowd, or dispute",
}

DETAIL_KINDS = {
    "包子": "physical-item",
    "咖啡": "physical-item",
    "泡面": "physical-item",
    "打车票": "tangible-token",
    "演唱会票": "tangible-token",
    "手机碎屏换新": "physical-item",
    "宠物急诊账单": "tangible-token",
    "宠物急诊押金": "tangible-token",
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
    "儿童安全座椅": "physical-item",
    "周末搬家加价": "experience-scene",
    "商标注册服务费": "experience-scene",
    "共享车押金找回失败": "experience-scene",
    "押金原路退回": "experience-scene",
    "停车超时补费": "experience-scene",
    "长辈寿宴礼金": "experience-scene",
    "婚纱礼服尾款": "experience-scene",
    "停车起步补费": "experience-scene",
    "设计图修改费": "experience-scene",
    "朋友突然还钱": "experience-scene",
    "同事离职礼物": "experience-scene",
    "前挡玻璃修复": "experience-scene",
    "房产过户测绘费": "experience-scene",
    "儿童乐园临时票": "experience-scene",
    "证件照加洗": "experience-scene",
    "全屋深度保洁": "experience-scene",
    "生日聚餐请客": "experience-scene",
    "营业执照代办": "experience-scene",
    "轮胎补气救援": "experience-scene",
    "优惠券过期反买": "experience-scene",
    "公证材料翻译": "experience-scene",
    "机场高速过路费": "experience-scene",
    "机场接送包车": "experience-scene",
    "办公室咖啡六杯": "experience-scene",
    "网约车取消费": "experience-scene",
    "牙科种植定金": "experience-scene",
    "跨境认证服务费": "experience-scene",
    "电脑主板维修": "experience-scene",
    "定制柜增项": "experience-scene",
    "仓储保管费": "experience-scene",
    "游戏账号申诉费": "experience-scene",
    "商务形象顾问": "experience-scene",
    "古玩鉴定服务费": "experience-scene",
    "航班延误赔付": "experience-scene",
    "少儿体检套餐": "experience-scene",
    "走错车道罚单": "experience-scene",
    "卫生间防水重做": "experience-scene",
    "退税突然到账": "experience-scene",
    "家电套装尾款": "experience-scene",
    "幼儿园手工作业材料": "physical-item",
    "专业教材整套": "physical-item",
    "行李延误临时采购": "experience-scene",
    "装修返工增项": "experience-scene",
    "小奖到账": "experience-scene",
    "保险理赔到账": "experience-scene",
    "周末自助早餐": "experience-scene",
    "双人奶茶外送": "experience-scene",
    "商务茶歇套餐": "experience-scene",
    "聚会香槟套餐": "experience-scene",
    "高峰打车加价": "experience-scene",
    "跨城顺风车补差": "experience-scene",
    "软件专业版续费": "experience-scene",
    "周末绘画体验课": "experience-scene",
    "牙齿矫正定金": "experience-scene",
    "私立学校校车费": "experience-scene",
    "海外研学团费": "experience-scene",
    "校园打印装订": "experience-scene",
    "语言考试报名费": "experience-scene",
    "异地考试住宿": "experience-scene",
    "面试西装租赁": "experience-scene",
    "职业证件照套餐": "experience-scene",
    "行业峰会门票": "experience-scene",
    "海外会议参会费": "experience-scene",
    "住院检查预缴": "experience-scene",
    "宠物住院押金": "experience-scene",
    "宠物心脏手术定金": "experience-scene",
    "返程机票改签": "experience-scene",
    "海外租车押金": "experience-scene",
    "全屋美缝追加": "experience-scene",
    "拍卖图录服务费": "experience-scene",
    "违停拖车保管费": "experience-scene",
    "电瓶道路救援": "experience-scene",
    "话费重复扣款退回": "experience-scene",
    "租房押金退回": "experience-scene",
    "年终奖补发": "experience-scene",
    "笔记本屏幕总成": "physical-item",
    "台式机显卡烧毁": "physical-item",
    "伴郎礼服押金": "experience-scene",
    "救护车费用": "experience-scene",
    "住院押金": "experience-scene",
    "机票改签费": "experience-scene",
    "欧洲自驾押金": "experience-scene",
    "宴会酒水押金": "experience-scene",
    "私立医院住院押金": "experience-scene",
    "行李超重补费": "experience-scene",
    "全家返程机票": "experience-scene",
    "婚车超时费用": "experience-scene",
    "超跑赛道事故押金": "experience-scene",
    "油画保税仓费用": "experience-scene",
    "车辆保险赔付": "experience-scene",
    "酒店取消退款": "experience-scene",
    "房屋退租押金到账": "experience-scene",
    "深夜牛肉面": "physical-item",
    "大杯手冲咖啡": "physical-item",
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
    "急修",
    "修理",
    "修复",
    "上门",
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


@dataclass(frozen=True)
class CatalogMetadata:
    """
    Keep only the existing game fields that can improve visual classification.

    A complete bootstrap row contains prices, weights, limits, and other gameplay data, but none
    of those values should influence image composition. Category, scene id, and tags already
    describe what the purchase represents, so retaining only these three fields keeps the image
    skill connected to the content model without turning it into a second copy of game logic.
    The empty value is also important: name-only callers must continue through the original name
    rules and therefore receive the same prompt they received before catalog support was added.
    """

    category: str = ""
    scene_id: str = ""
    tags: tuple[str, ...] = ()


@dataclass(frozen=True)
class ItemInput:
    """Carry one validated input row from parsing to prompt generation."""

    item_id: str
    item_name: str
    metadata: CatalogMetadata = CatalogMetadata()


# These values come from the existing PostgreSQL content vocabulary. They do not decide the
# exact picture by themselves. They only resolve the ambiguous case where a name such as
# "商务形象顾问" contains no document word and no physical product noun. Explicit tickets,
# bills, known products, and the old name rules still run first, which preserves old invocations.
STRUCTURED_SERVICE_CATEGORIES = frozenset(
    {
        "交通",
        "平台规则",
        "灾难维修",
        "社交压力",
        "亲子教育",
        "教育考试",
        "职场晋升",
        "健康",
        "宠物",
        "法律行政",
        "旅行",
        "大件现实",
        "富人体验",
        "高端误操作",
        "高端隐形成本",
    }
)
STRUCTURED_SERVICE_SCENE_IDS = frozenset(
    {
        "repair-week",
        "car-owner-day",
        "travel-chaos",
        "home-renovation",
        "pet-er",
        "admin-window",
        "auction-night",
        "wedding-season",
        "social-pressure",
        "parenting-week",
        "office-promotion",
        "midlife-checkup",
        "luxury-afterparty",
        "campus-wallet",
        "masked-party",
        "hospital-corridor",
        "lucky-counter",
    }
)
STRUCTURED_SERVICE_TAGS = frozenset(
    {
        "repair",
        "medical",
        "education",
        "admin",
        "legal",
        "travel",
        "hotel",
        "flight",
        "renovation",
        "rework",
        "wedding",
        "luxury",
        "service-fee",
        "auction",
        "career",
        "subscription",
        "traffic",
        "platform",
        "income",
        "refund",
        "compensation",
        "insurance",
        "pet",
        "social",
        "family",
    }
)
STRUCTURED_STRONG_SERVICE_TAGS = STRUCTURED_SERVICE_TAGS.difference({"social", "family"}).union(
    {"batch", "resale"}
)
STRUCTURED_SERVICE_NAME_HINTS = (
    "外送",
    "包间",
    "聚餐",
    "请客",
    "自助",
    "私厨",
    "庆功宴",
    "晚宴",
    "宴会",
    "包桌",
    "包场",
    "月卡",
    "茶歇",
    "下午茶",
    "拼盘",
    "四人餐",
    "六杯",
    "演唱会",
)
STRUCTURED_PHYSICAL_CATEGORIES_BY_TAG = (
    ("appliance", "appliance"),
    ("food", "food"),
    ("snack", "food"),
    ("drink", "drink"),
    ("digital", "electronics"),
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


def default_image_gen_script() -> Path:
    """Return the image generator installed under the current user's Codex home."""
    configured_home = os.environ.get("CODEX_HOME", "").strip()
    codex_home = Path(configured_home).expanduser() if configured_home else Path.home() / ".codex"
    return codex_home / "skills" / ".system" / "imagegen" / "scripts" / "image_gen.py"


def pillow_available() -> bool:
    """Check image post-processing before a paid generation request starts."""
    return importlib.util.find_spec("PIL") is not None


def clean_item(value: str) -> str:
    value = value.strip().lstrip("\ufeff")
    value = re.sub(r"^\s*[\d一二三四五六七八九十]+[.、)]\s*", "", value)
    return re.sub(r"\s+", " ", value)


def clean_item_id(value: object) -> str:
    item_id = str(value).strip()
    if not ITEM_ID_PATTERN.fullmatch(item_id):
        raise ValueError(
            f"invalid item id {item_id!r}; use 1-128 ASCII letters, digits, dots, underscores, or hyphens"
        )
    return item_id


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


def catalog_metadata_describes_service(metadata: CatalogMetadata) -> bool:
    """
    Return whether structured game content describes a real service or experience.

    Scene ids and tags are checked in addition to the human-facing category because older content
    rows sometimes use a broad category such as "中奖捡漏" while their tags carry the useful
    `income` or `auction` meaning. All comparisons use the repository's current controlled
    vocabulary. An unknown future value simply returns false and falls back to name classification.
    """

    normalized_tags = {tag.casefold() for tag in metadata.tags}
    return (
        metadata.category in STRUCTURED_SERVICE_CATEGORIES
        or metadata.scene_id in STRUCTURED_SERVICE_SCENE_IDS
        or not normalized_tags.isdisjoint(STRUCTURED_SERVICE_TAGS)
    )


def catalog_metadata_strongly_describes_service(metadata: CatalogMetadata) -> bool:
    """Return whether a specific service tag should win over a broad physical tag."""

    normalized_tags = {tag.casefold() for tag in metadata.tags}
    return not normalized_tags.isdisjoint(STRUCTURED_STRONG_SERVICE_TAGS)


def has_structured_catalog_metadata(metadata: CatalogMetadata) -> bool:
    """Distinguish a game catalog row from the empty metadata used by legacy name input."""

    return bool(metadata.category or metadata.scene_id or metadata.tags)


def structured_physical_category(metadata: CatalogMetadata) -> str:
    """Map high-confidence physical tags to an existing manifest category."""

    normalized_tags = {tag.casefold() for tag in metadata.tags}
    for tag, category in STRUCTURED_PHYSICAL_CATEGORIES_BY_TAG:
        if tag in normalized_tags:
            return category
    return ""


def subject_for(item: str, metadata: CatalogMetadata = CatalogMetadata()) -> str:
    if item in DETAILS:
        return DETAILS[item]
    if item in SERVICE_SCENE_DETAILS:
        return SERVICE_SCENE_DETAILS[item]
    kind = subject_kind(item, metadata)
    if kind == "tangible-token":
        if has_hint(item, ("票", "券", "ticket", "boarding pass")):
            return f"one real ticket or pass representing {item}, shown alone with a complete outline and no readable barcode or private information"
        return f"real receipt or payment slip representing {item}, placed on an ordinary table, no readable private information"
    if kind == "experience-scene":
        return f"one cinematic real-world service experience representing {item}, centered on one immediately recognizable place or action with limited supporting context"
    return f"named real-world product {item}, shown alone with its authentic market shape and defining material texture"


def subject_kind(item: str, metadata: CatalogMetadata = CatalogMetadata()) -> str:
    if item in DETAIL_KINDS:
        return DETAIL_KINDS[item]
    if has_hint(item, DOCUMENT_HINTS):
        return "tangible-token"
    if item in SERVICE_SCENE_DETAILS or has_hint(item, SERVICE_SCENE_HINTS):
        return "experience-scene"
    if has_structured_catalog_metadata(metadata) and has_hint(item, STRUCTURED_SERVICE_NAME_HINTS):
        return "experience-scene"
    if catalog_metadata_strongly_describes_service(metadata):
        return "experience-scene"
    # A physical tag is more specific than broad context tags such as `social` or `home`. For
    # example, bottled water may be bought in a social setting, but its image should still show
    # the drink itself. The name-based service check above remains stronger, so a food package,
    # repair, delivery, or subscription still becomes a service scene when its wording says so.
    if structured_physical_category(metadata):
        return "physical-item"
    if catalog_metadata_describes_service(metadata):
        return "experience-scene"
    return "physical-item"


def subject_category_for(item: str, metadata: CatalogMetadata = CatalogMetadata()) -> str:
    normalized = item.casefold()
    # DETAIL_KINDS is the most具体的人工判断。例如“押金原路退回”虽然名称中有
    # “押金”，但我们已经明确要求它展示顾客收到退款的服务场景，而不是展示一张押金
    # 单据。因此这里先读取 subject_kind，并让显式的 experience-scene 覆盖后面的名称
    # 关键词。没有进入 DETAILS_KINDS 的旧卡仍会继续使用原来的票据关键词和数据库标签，
    # 所以这项调整只修正人工指定过的例外，不会把所有含“押金”或“补费”的卡批量改类。
    kind = subject_kind(item, metadata)
    if kind == "experience-scene":
        return "service-scene"
    if any(hint in normalized for hint in ("账单", "罚单", "发票", "收据", "押金", "缴费", "费用", "invoice", "receipt", "bill")):
        return "bill-or-receipt"
    if any(hint in normalized for hint in ("票", "券", "通行证", "登机牌", "ticket", "boarding pass")):
        return "ticket"
    if kind == "tangible-token":
        return "service-token"
    # Database tags are curated alongside each item and are therefore more reliable than a
    # substring collision in the display name. This prevents "火锅四人餐" from becoming cookware,
    # "大杯手冲咖啡" from becoming empty drinkware, and "笔记本屏幕总成" from becoming stationery.
    metadata_category = structured_physical_category(metadata)
    if metadata_category:
        return metadata_category
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


def visual_archetype_for(item: str, metadata: CatalogMetadata = CatalogMetadata()) -> str:
    category = subject_category_for(item, metadata)
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


def style_profile_for(
    item: str,
    requested: str = "auto",
    style_seed: str = DEFAULT_STYLE_SEED,
    metadata: CatalogMetadata = CatalogMetadata(),
) -> str:
    if requested != "auto":
        if requested != "iphone-realphoto" and requested not in STYLE_PROFILES:
            raise ValueError(f"unknown style profile: {requested}")
        return requested
    if not style_seed:
        raise ValueError("style_seed must not be empty")
    pool = STYLE_POOLS_BY_CATEGORY.get(subject_category_for(item, metadata), STYLE_PROFILE_ORDER)
    digest = hashlib.sha256(f"{style_seed}\0{item}".encode("utf-8")).digest()
    return pool[int.from_bytes(digest[:8], "big") % len(pool)]


def palette_profile_for(item: str, style_seed: str = DEFAULT_STYLE_SEED) -> str:
    if not style_seed:
        raise ValueError("style_seed must not be empty")
    digest = hashlib.sha256(f"palette\0{style_seed}\0{item}".encode("utf-8")).digest()
    return PALETTE_PROFILE_ORDER[int.from_bytes(digest[:8], "big") % len(PALETTE_PROFILE_ORDER)]


def composition_cue_for(item: str, metadata: CatalogMetadata = CatalogMetadata()) -> str:
    archetype = visual_archetype_for(item, metadata)
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


def prompt_for(
    item: str,
    requested_style: str = "auto",
    style_seed: str = DEFAULT_STYLE_SEED,
    metadata: CatalogMetadata = CatalogMetadata(),
) -> str:
    style_profile = style_profile_for(item, requested_style, style_seed, metadata)
    if style_profile == "iphone-realphoto":
        return (
            f"Close-up iPhone snapshot of a single {subject_for(item, metadata)}, "
            f"landscape horizontal frame for a wide marketplace item tile art crop, centered product silhouette, "
            f"safe empty margin near all edges for deterministic UI overlay, one dominant item only, casual handheld composition, "
            f"no text, no border, no decorative frame, no dominant logo, no extra props competing with the subject, {FIXED_BASE}"
        )
    category = subject_category_for(item, metadata)
    archetype = visual_archetype_for(item, metadata)
    palette = palette_profile_for(item, style_seed)
    if archetype == "experience-scene":
        return (
            f"Single-image game UI artwork for {subject_for(item, metadata)}, {CATEGORY_CUES[category]}. "
            f"Wide horizontal 340:156 item-card art composition using the full wide canvas, one dominant visual focus only, {composition_cue_for(item, metadata)}, "
            f"{VISUAL_ARCHETYPE_CUES[archetype]}. {MASTER_SCENE_ART_DIRECTION}. "
            f"Use {PALETTE_PROFILES[palette]} mainly in environmental color and light accents while preserving plausible local colors. "
            f"Apply this controlled surface treatment: {STYLE_PROFILES[style_profile]['prompt']}. "
            f"Keep a narrow 6 percent safe edge margin and one coherent scene. No voucher, receipt, invoice, email, membership card, confirmation page, or document unless explicitly named by the input. "
            f"No montage, no collage, no split panel, no crowd, no unrelated secondary action, no readable personal data, no added typography, "
            f"no card border, no game UI, no rarity badge, no signature, no watermark"
        )
    return (
        f"Single-item game UI artwork of a single {subject_for(item, metadata)}, {CATEGORY_CUES[category]}. "
        f"Wide horizontal 340:156 item-card art composition, one dominant item only, {composition_cue_for(item, metadata)}, "
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
    item_id: str = "",
    metadata: CatalogMetadata = CatalogMetadata(),
) -> dict[str, object]:
    item_errors = validate_item_name(item)
    if item_errors:
        raise ValueError(f"{item}: {'; '.join(item_errors)}")
    style_profile = style_profile_for(item, requested_style, style_seed, metadata)
    prompt = prompt_for(item, style_profile, style_seed, metadata)
    archetype = visual_archetype_for(item, metadata)
    errors = validate_prompt(prompt, style_profile, style_profile != "iphone-realphoto", archetype)
    if errors:
        raise ValueError(f"{item}: {'; '.join(errors)}")
    visual_grade = visual_grade_for(item, requested_grade)
    record = {
        "schema_version": LEGACY_SCHEMA_VERSION if style_profile == "iphone-realphoto" else SCHEMA_VERSION,
        "style_signature": style_signature_for(style_profile),
        "index": index,
        "item_name": item,
        "subject_kind": subject_kind(item, metadata),
        "subject": subject_for(item, metadata),
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
    if item_id:
        record["item_id"] = clean_item_id(item_id)
    if style_profile != "iphone-realphoto":
        record.update(
            {
                "subject_category": subject_category_for(item, metadata),
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


def validate_record(
    record: dict[str, object],
    seen_indexes: set[int],
    seen_item_ids: Optional[set[str]] = None,
) -> list[str]:
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
    item_id = str(record.get("item_id", "")).strip()
    if item_id:
        try:
            item_id = clean_item_id(item_id)
        except ValueError as exc:
            errors.append(str(exc))
        else:
            if seen_item_ids is not None and item_id in seen_item_ids:
                errors.append(f"duplicate item_id: {item_id}")
            if seen_item_ids is not None:
                seen_item_ids.add(item_id)
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
    seen_item_ids: set[str] = set()
    failures: list[str] = []
    for row_number, record in enumerate(records, 1):
        errors = validate_record(record, seen_indexes, seen_item_ids)
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
    item_id = str(record.get("item_id", "")).strip()
    if item_id:
        return f"{clean_item_id(item_id)}.{output_format}"
    return f"{int(str(record['index'])):04d}.{output_format}"


def write_image2_jobs(records: list[dict[str, object]], path: Path, output_format: str) -> None:
    jobs = []
    for record in records:
        jobs.append(
            {
                "prompt": record["prompt"],
                "out": image_output_name(record, output_format),
                "fields": {
                    "item_id": record.get("item_id"),
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
    script = Path(args.image_gen_script).expanduser() if args.image_gen_script else default_image_gen_script()
    env = build_child_env(args.base_url)
    status = {
        "python": sys.executable,
        "pillow_available": pillow_available(),
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
    record_by_item_id = {}
    for record in records:
        item_id = str(record.get("item_id", "")).strip()
        if item_id:
            record_by_item_id[item_id] = record
        try:
            record_by_index[int(str(record["index"]))] = record
        except Exception:
            continue
    rendered: list[str] = []
    suffix = "." + output_format.lower()
    for position, path in enumerate(sorted(images_dir.glob(f"*{suffix}")), 1):
        record = record_by_item_id.get(path.stem)
        try:
            index = int(path.stem)
        except ValueError:
            index = position
        if record is None:
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
    record_by_item_id = {
        str(record["item_id"]): record
        for record in records
        if str(record.get("item_id", "")).strip()
    }
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
            record = record_by_item_id.get(path.stem)
            try:
                index = int(path.stem)
            except ValueError:
                index = start + slot + 1
            if record is None:
                record = record_by_index.get(index, {})
            style = str(record.get("style_profile", "unknown"))
            archetype = str(record.get("visual_archetype", "unknown"))
            identity = str(record.get("item_id") or f"{index:04d}")
            draw.rectangle((x, y + FRONTEND_CARD_HEIGHT, x + cell_width, y + cell_height), fill=(20, 22, 28))
            draw.text((x + 6, y + FRONTEND_CARD_HEIGHT + 5), f"{identity}  {style}  {archetype}", fill=(232, 235, 240))
        output = review_out / f"contact-{page_index:04d}.png"
        sheet.save(output)
        written.append(str(output))
    return written


def run_image2_batch(records: list[dict[str, object]], args: argparse.Namespace) -> None:
    script = Path(args.image_gen_script).expanduser() if args.image_gen_script else default_image_gen_script()
    if not script.exists():
        raise FileNotFoundError(f"image_gen.py not found: {script}")
    post_processing_enabled = not (
        args.no_normalize_images
        and args.no_render_card_frames
        and args.no_render_review_sheets
    )
    if not args.dry_run and post_processing_enabled and not pillow_available():
        raise RuntimeError(
            "Pillow is required before image generation because normalization or review rendering is enabled. "
            "Run this command with the Codex bundled Python or install Pillow in the selected Python environment."
        )
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


def clean_optional_catalog_text(value: object, field_name: str, row_number: int) -> str:
    """Validate an optional catalog text field while accepting a JSON null value."""

    if value is None:
        return ""
    if not isinstance(value, str):
        raise ValueError(f"catalog row {row_number}: {field_name} must be a string or null")
    return re.sub(r"\s+", " ", value.strip())


def clean_catalog_tags(value: object, row_number: int) -> tuple[str, ...]:
    """
    Validate the JSON tags array and remove duplicate or blank values without reordering it.

    Tags are controlled content labels, not free-form prompt text. Rejecting objects and numbers
    here prevents a malformed bootstrap row from being silently converted into a misleading
    string such as a Python dictionary representation later in the classifier.
    """

    if value is None:
        return ()
    if not isinstance(value, list):
        raise ValueError(f"catalog row {row_number}: tags must be an array or null")
    tags: list[str] = []
    seen: set[str] = set()
    for tag_number, tag in enumerate(value, 1):
        if not isinstance(tag, str):
            raise ValueError(f"catalog row {row_number}: tag {tag_number} must be a string")
        cleaned = re.sub(r"\s+", " ", tag.strip())
        if cleaned and cleaned not in seen:
            seen.add(cleaned)
            tags.append(cleaned)
    return tuple(tags)


def parse_catalog_items(raw: str) -> list[ItemInput]:
    """
    Read either a complete bootstrap JSON object, a JSON array, or JSONL rows.

    The game database already owns the stable item id, while display names can be edited later.
    Keeping both values here lets generated files follow the database identity instead of a fragile
    batch position such as 0001.png. Existing name-only input remains available through --file,
    --items, or plain stdin, so older calls continue to produce their original numbered files.
    """
    text = raw.strip()
    if not text:
        raise ValueError("catalog input is empty")

    try:
        payload: object = json.loads(text)
    except json.JSONDecodeError:
        rows: list[object] = []
        for line_number, line in enumerate(text.splitlines(), 1):
            if not line.strip():
                continue
            try:
                rows.append(json.loads(line))
            except json.JSONDecodeError as exc:
                raise ValueError(f"catalog line {line_number}: invalid json: {exc}") from exc
        payload = rows

    if isinstance(payload, dict) and "items" in payload:
        source = payload["items"]
    elif isinstance(payload, dict):
        source = [payload]
    else:
        source = payload
    if not isinstance(source, list):
        raise ValueError("catalog must be a JSON array, JSONL rows, or an object containing an items array")

    items: list[ItemInput] = []
    seen_item_ids: set[str] = set()
    for row_number, value in enumerate(source, 1):
        if not isinstance(value, dict):
            raise ValueError(f"catalog row {row_number}: expected an object")
        item_id = clean_item_id(value.get("id", ""))
        item_name = clean_item(str(value.get("name", "")))
        if not item_name:
            raise ValueError(f"catalog row {row_number}: name must not be empty")
        errors = validate_item_name(item_name)
        if errors:
            raise ValueError(f"catalog row {row_number} ({item_id}): {'; '.join(errors)}")
        if item_id in seen_item_ids:
            raise ValueError(f"catalog row {row_number}: duplicate item id {item_id}")
        seen_item_ids.add(item_id)
        metadata = CatalogMetadata(
            category=clean_optional_catalog_text(value.get("category"), "category", row_number),
            scene_id=clean_optional_catalog_text(value.get("sceneId"), "sceneId", row_number),
            tags=clean_catalog_tags(value.get("tags"), row_number),
        )
        items.append(ItemInput(item_id=item_id, item_name=item_name, metadata=metadata))

    return items


def read_item_inputs(args: argparse.Namespace) -> list[ItemInput]:
    inputs: list[ItemInput] = []
    if args.catalog_file:
        catalog_text = sys.stdin.read() if args.catalog_file == "-" else Path(args.catalog_file).read_text(encoding="utf-8-sig")
        inputs.extend(parse_catalog_items(catalog_text))

    raw: list[str] = []
    if args.file:
        raw.extend(Path(args.file).read_text(encoding="utf-8").splitlines())
    if args.items:
        raw.extend(args.items)
    if not inputs and not raw and not sys.stdin.isatty():
        raw.extend(sys.stdin.read().splitlines())
    for item in (clean_item(value) for value in raw):
        if item:
            inputs.append(ItemInput(item_id="", item_name=item))
    return inputs


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
    parser.add_argument(
        "--catalog-file",
        help="Bootstrap JSON, JSON array, or JSONL catalog with stable id and name fields; use - for stdin",
    )
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
        seen_item_ids: set[str] = set()
        failures = []
        for row_number, record in enumerate(records, 1):
            errors = validate_record(record, seen_indexes, seen_item_ids)
            if errors:
                failures.append(f"row {row_number}: {'; '.join(errors)}")
        if failures:
            raise ValueError("\n".join(failures))
    else:
        item_inputs = read_item_inputs(args)
        if not item_inputs:
            raise SystemExit("No items provided.")
        records = [
            record_for(
                index,
                item_input.item_name,
                args.visual_grade,
                args.style_profile,
                args.style_seed,
                item_input.item_id,
                item_input.metadata,
            )
            for index, item_input in enumerate(item_inputs, 1)
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
        output = "\n".join(
            f"{record.get('item_id') or record['index']}. {record['prompt']}"
            for record in records
        )
    if args.out:
        Path(args.out).write_text(output + "\n", encoding="utf-8")
        return
    print(output)


if __name__ == "__main__":
    main()
