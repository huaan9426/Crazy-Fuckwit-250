---
name: draw-iphone
description: Generate separate physical-item or experiential-service game UI images and gpt-image-2 batches for Crazy-Fuckwit-250 through one cohesive game-art direction and six deterministic surface treatments, with backward-compatible earlier mixed-art and iPhone realism modes. Use when the user says /draw iphone, asks for item-card art, supplies product, food, drinkware, appliance, ticket, receipt, bill, subscription, booking, delivery, rental, service, or commodity names, or wants vivid but reproducible item image batches.
---

# Draw iPhone

Turn item names into separate, high-impact game item images.

## Boundary

Generate prompts and images only. Do not modify game code, database schema, UI rendering, prices, gameplay, or asset field consumers.

Call gpt-image-2 only when the user says /draw iphone or explicitly requests image generation.

Keep one input line per prompt and image. Physical goods use one dominant item. Experiential services use one coherent wide scene with one dominant action or focal cluster. Never merge several input lines into one image.

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

For experiential services, replace the product hero with one readable experience, place, or action. Use foreground, midground, and background depth across the full canvas. Allow only essential people and props; prohibit crowds, collages, split panels, and unrelated secondary actions.

Use these six profiles only as subordinate surface treatments:

- fauvist-paint
- pop-art-print
- constructivist-poster
- baroque-still-life
- hyperreal-ad
- retro-east-asian-ad

Assign style and palette profiles with stable SHA-256 selection. The same item and seed must receive the same treatment after retries or batch reordering.

Do not use runtime randomness for production. Change --style-seed only to create a deliberate new art-direction pass.

Classify physical form separately from semantic category. New rows use one of: `food-closeup`, `tall-vessel`, `handheld-electronics`, `appliance`, `cookware`, `soft-apparel`, `flat-document`, `experience-scene`, `small-hard-good`, or `hero-product`.

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

1. Read item names from the user, a UTF-8 file, stdin, or a game bootstrap/catalog JSON file.
2. Preserve item order and reject corrupted input. When catalog rows include `id`, keep that stable game id through the manifest and output filename. Catalog rows may also provide `category`, `sceneId`, and `tags`; use those existing game fields to resolve ambiguous service scenes, while explicit document words and physical product tags keep their more specific representation.
3. Convert every input into one physical subject, explicit document token, or experiential service scene.
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

Generate from the game's bootstrap JSON and name outputs by `item.id`:

    python ".agents/skills/draw-iphone/scripts/generate_iphone_photo_prompts.py" --catalog-file "bootstrap.json" --format jsonl

Read a bootstrap object, item array, or JSONL catalog from stdin:

    curl -s "http://localhost:3001/api/content/bootstrap" | python ".agents/skills/draw-iphone/scripts/generate_iphone_photo_prompts.py" --catalog-file - --format jsonl

Force one style:

    python ".agents/skills/draw-iphone/scripts/generate_iphone_photo_prompts.py" --items "包子" "咖啡" --style-profile pop-art-print

Use legacy iPhone realism:

    python ".agents/skills/draw-iphone/scripts/generate_iphone_photo_prompts.py" --items "包子" "咖啡" --style-profile iphone-realphoto

Check the image2 channel:

    python ".agents/skills/draw-iphone/scripts/generate_iphone_photo_prompts.py" --check-image2-channel

The script resolves `image_gen.py` from `$CODEX_HOME/skills/.system/imagegen/scripts/` or `~/.codex/skills/.system/imagegen/scripts/` on macOS, Linux, and Windows. Before a paid batch, require `image_gen_script_exists`, `pillow_available`, and `openai_api_key_available` to be true. If the normal `python3` has no Pillow, use the Codex bundled Python returned by the workspace dependency loader.

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

Catalog input writes `<item-id>.png` so the file can be joined to the frontend with the existing game item id. A complete bootstrap catalog also uses `category`, `sceneId`, and `tags` during classification, but does not copy gameplay prices or weights into the art manifest. Legacy name-only input keeps the numbered `0001.png` behavior and the original name-only classifier.

The default preview grade is neutral bronze. Do not infer gameplay rarity from a brand or product name; pass an explicit grade or use game-owned metadata when another preview grade is required.

## V2 Manifest

New mixed-style rows use `item-art-prompt-manifest/v2` with the `CF250_ITEM_ART_COHESIVE_V3` signature and include:

- style_signature
- item_id when input came from a structured game catalog
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
- Keep exactly one dominant physical item, or one coherent service scene with one dominant focal cluster.
- Use a document only when the input explicitly names a ticket, pass, bill, invoice, receipt, fine, voucher, or boarding pass.
- Render subscriptions, bookings, memberships, delivery, rentals, fitness, photography, hotels, taxi rides, repairs, and similar services as the real experience rather than a confirmation document.
- Preserve real-world identity, silhouette, proportions, function, defining materials, controls, ports, lids, handles, and mechanisms where present.
- Keep the selected treatment subordinate to the product.
- Use archetype-specific framing for food, vessels, handheld electronics, appliances, cookware, apparel, documents, and small hard goods.
- Keep a narrow safe edge margin for UI overlay.
- In physical-item images, do not add people, hands, or competing props. In service scenes, allow only essential people and supporting objects with non-identifying faces.
- Do not generate card frames, UI, rarity decoration, added typography, readable private data, dominant logos, signatures, or watermarks.
- For an exact branded model, use a product reference image when available. Text-only generation cannot guarantee model fidelity.
- Review every batch in the generated true-size 340x156 contact sheet before acceptance.
- Reject failed rows individually; never merge failures into one prompt.
