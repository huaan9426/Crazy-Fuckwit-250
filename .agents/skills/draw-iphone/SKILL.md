---
name: draw-iphone
description: Generate separate single-item game UI art prompts and gpt-image-2 batches for Crazy-Fuckwit-250 through one cohesive game-art direction and six deterministic surface treatments, with backward-compatible earlier mixed-art and iPhone realism modes. Use when the user says /draw iphone, asks for item-card art, supplies product, food, drinkware, appliance, ticket, receipt, bill, service, or commodity names, or wants vivid but reproducible item image batches.
---

# Draw iPhone

Turn item names into separate, high-impact game item images.

## Boundary

Generate prompts and images only. Do not modify game code, database schema, UI rendering, prices, gameplay, or asset field consumers.

Call gpt-image-2 only when the user says /draw iphone or explicitly requests image generation.

Keep one input item per prompt and one dominant item per image. Never merge a list into one image.

Keep card borders, title bars, prices, stats, rarity badges, and other UI in the deterministic overlay layer.

## Default Art Direction

Use `CF250_GAME_ITEM_ART_BIBLE_V1` for every new mixed-style image:

- one product is the only hero;
- dynamic three-quarter or strong diagonal view;
- complete thumbnail-readable silhouette;
- cinematic commercial realism with a polished painted finish;
- localized key and rim light with readable midtones and open shadows;
- two broad background color masses and one restrained accent;
- no competing props, scene clutter, card UI, or generated frame.

Use these six profiles only as subordinate surface treatments:

- fauvist-paint
- pop-art-print
- constructivist-poster
- baroque-still-life
- hyperreal-ad
- retro-east-asian-ad

Assign style and palette profiles with stable SHA-256 selection. The same item and seed must receive the same treatment after retries or batch reordering.

Do not use runtime randomness for production. Change --style-seed only to create a deliberate new art-direction pass.

Classify physical form separately from semantic category. New rows use one of: `food-closeup`, `tall-vessel`, `handheld-electronics`, `appliance`, `cookware`, `soft-apparel`, `flat-document`, `small-hard-good`, or `hero-product`.

For tickets, bills, receipts, and service tokens, use the safer four-style subset defined in `references/iphone-photo-rules.md`.

Use --style-profile NAME to force one style for a test or curated batch. Use --style-profile iphone-realphoto for the original CF250_IPHONE_REALPHOTO_V1 behavior.

## Shortcut

When a message starts with /draw iphone, treat the remaining non-empty lines as item names, preserve their order, and generate one separate image2 job per line.

    /draw iphone
    光明冰砖
    上海英雄钢笔
    演唱会票
    手机碎屏换新

## Workflow

1. Read item names from the user, a UTF-8 file, or stdin.
2. Preserve item order and reject corrupted input.
3. Convert every item into one concrete physical subject or tangible service token.
4. Classify its semantic category and visual archetype.
5. Select stable style and palette profiles unless the user forces a style.
6. Build and validate one prompt row per item.
7. Generate one image2 job per row.
8. Normalize raw art into images/ at 2040x936.
9. Render optional thin-frame previews into framed/.
10. Render true-size 340x156 batch contact sheets into review/.
11. Keep prompt_manifest.jsonl and image2_jobs.jsonl for production traceability.

Read references/iphone-photo-rules.md before changing style profiles, category rules, prompt QA, or manifest fields.

## Script

List production styles:

    python ".agents/skills/draw-iphone/scripts/generate_iphone_photo_prompts.py" --list-styles

Generate a stable mixed-style prompt batch:

    python ".agents/skills/draw-iphone/scripts/generate_iphone_photo_prompts.py" --items "包子" "咖啡" "上海英雄钢笔" "演唱会票"

Force one style:

    python ".agents/skills/draw-iphone/scripts/generate_iphone_photo_prompts.py" --items "包子" "咖啡" --style-profile pop-art-print

Use legacy iPhone realism:

    python ".agents/skills/draw-iphone/scripts/generate_iphone_photo_prompts.py" --items "包子" "咖啡" --style-profile iphone-realphoto

Check the image2 channel:

    python ".agents/skills/draw-iphone/scripts/generate_iphone_photo_prompts.py" --check-image2-channel

Generate images:

    python ".agents/skills/draw-iphone/scripts/generate_iphone_photo_prompts.py" --file ".\items.txt" --image2-out ".\draw-iphone-run"

Dry-run without generation cost:

    python ".agents/skills/draw-iphone/scripts/generate_iphone_photo_prompts.py" --file ".\items.txt" --image2-out ".\draw-iphone-run" --dry-run

Validate outputs:

    python ".agents/skills/draw-iphone/scripts/generate_iphone_photo_prompts.py" --validate-manifest ".\draw-iphone-run\prompt_manifest.jsonl"
    python ".agents/skills/draw-iphone/scripts/generate_iphone_photo_prompts.py" --validate-images ".\draw-iphone-run\images"

Render review sheets for an existing run:

    python ".agents/skills/draw-iphone/scripts/generate_iphone_photo_prompts.py" --manifest ".\draw-iphone-run\prompt_manifest.jsonl" --render-review-sheets ".\draw-iphone-run\images"

Use UTF-8 files for large Chinese batches on Windows.

## Image2 Contract

- Model: gpt-image-2
- Quality: medium
- Request size: 2048x944
- Final asset size: 2040x936
- Frontend reference: 340x156
- Concurrency: 2
- Output: png
- Raw art: images/
- Thin-frame preview: framed/
- True-size batch review: review/contact-*.png

Keep normalization enabled in production.

The default preview grade is neutral bronze. Do not infer gameplay rarity from a brand or product name; pass an explicit grade or use game-owned metadata when another preview grade is required.

## V2 Manifest

New mixed-style rows use `item-art-prompt-manifest/v2` with the `CF250_ITEM_ART_COHESIVE_V3` signature and include:

- style_signature
- style_profile
- style_family
- style_assignment
- style_seed
- art_direction_signature
- visual_archetype
- palette_profile
- subject_category
- existing item, subject, size, frame, prompt, and QA fields

Existing `CF250_ITEM_ART_MIXED_V2` rows and old `iphone-product-photo-prompt-manifest/v1` rows remain valid.

## Quality Rules

- One input line equals one prompt, one job, and one image.
- Keep exactly one dominant item and no duplicate product.
- Preserve real-world identity, silhouette, proportions, function, defining materials, controls, ports, lids, handles, and mechanisms where present.
- Keep the selected treatment subordinate to the product.
- Use archetype-specific framing for food, vessels, handheld electronics, appliances, cookware, apparel, documents, and small hard goods.
- Keep a narrow safe edge margin for UI overlay.
- Do not generate card frames, UI, rarity decoration, added typography, readable private data, dominant logos, signatures, or watermarks.
- For an exact branded model, use a product reference image when available. Text-only generation cannot guarantee model fidelity.
- Review every batch in the generated true-size 340x156 contact sheet before acceptance.
- Reject failed rows individually; never merge failures into one prompt.
