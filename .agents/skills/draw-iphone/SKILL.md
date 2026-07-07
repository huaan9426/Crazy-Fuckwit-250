---
name: draw-iphone
description: Generate single-item photorealistic iPhone product photo prompts and gpt-image-2 image batches for Crazy-Fuckwit-250 item images. Use when the user says /draw iphone, asks to draw iPhone product photos, provides batches of object, food, product, ticket, receipt, bill, service, or commodity names, or wants consistent no-filter iPhone snapshot prompts or images.
---

# Draw iPhone

Turn item names into consistent single-subject iPhone photo prompts.

## Boundary

Generate image prompts and, when the user says `/draw iphone` or explicitly asks to generate images, call gpt-image-2 through the configured image2 channel.

Do not modify game code, database schema, UI rendering, item prices, gameplay rules, or asset fields. Do not call image APIs unless the user says `/draw iphone` or otherwise explicitly asks for image generation.

Do not add decorative game-card styling, artificial mood effects, floating props, dramatic warning effects, rendered artwork language, or joke/satire framing.

## Shortcut

Use this skill when the user starts a message with `/draw iphone`.

Treat the remaining non-empty lines as item names. Generate one separate image per item through image2. Do not merge multiple item names into one prompt or image.

Example:

```text
/draw iphone
光明冰砖
国际饭店蝴蝶酥
蜂花檀香皂
```

## Workflow

1. Read item names from the user, a UTF-8 file, or stdin.
2. Preserve item order.
3. Convert each item into one concrete real-world photographic subject.
4. Keep exactly one dominant item in the frame.
5. Apply the fixed iPhone realism base in `references/iphone-photo-rules.md`.
6. Add deterministic UI metadata: `visual_grade`, `frame_style`, `photo_role`, and `ui_overlay_required`.
7. For `/draw iphone`, create an output run directory and run image2 with one job per item.
8. Normalize raw photos into `images/`, then render deterministic card-frame previews into `framed/`.
9. For production batches, keep `prompt_manifest.jsonl` and `image2_jobs.jsonl` with the same style signature for every row.
10. Output a numbered prompt list only when the user asks for prompts instead of images.

## Script

Use the script for batches:

```powershell
python ".agents/skills/draw-iphone/scripts/generate_iphone_photo_prompts.py" --items "包子" "咖啡" "手机碎屏换新"
```

Check the image2 channel:

```powershell
python ".agents/skills/draw-iphone/scripts/generate_iphone_photo_prompts.py" --check-image2-channel
```

Generate images with image2:

```powershell
python ".agents/skills/draw-iphone/scripts/generate_iphone_photo_prompts.py" --items "包子" "咖啡" --image2-out ".\draw-iphone-run"
```

Dry-run image2 without spending generation calls:

```powershell
python ".agents/skills/draw-iphone/scripts/generate_iphone_photo_prompts.py" --items "包子" "咖啡" --image2-out ".\draw-iphone-run" --dry-run
```

Validate generated image dimensions:

```powershell
python ".agents/skills/draw-iphone/scripts/generate_iphone_photo_prompts.py" --validate-images ".\draw-iphone-run\images"
```

Normalize an existing image directory in place:

```powershell
python ".agents/skills/draw-iphone/scripts/generate_iphone_photo_prompts.py" --normalize-images ".\draw-iphone-run\images"
```

Render deterministic card-frame previews from existing images:

```powershell
python ".agents/skills/draw-iphone/scripts/generate_iphone_photo_prompts.py" --manifest ".\draw-iphone-run\prompt_manifest.jsonl" --render-card-frames ".\draw-iphone-run\images" --frame-out ".\draw-iphone-run\framed"
```

Use a file:

```powershell
python ".agents/skills/draw-iphone/scripts/generate_iphone_photo_prompts.py" --file ".\items.txt"
```

Use UTF-8 files or `--items` for Chinese names on Windows. Reject corrupted input such as repeated question marks.

Use a production manifest:

```powershell
python ".agents/skills/draw-iphone/scripts/generate_iphone_photo_prompts.py" --file ".\items.txt" --format jsonl --out ".\iphone-photo-prompts.jsonl"
```

Generate images from an existing manifest:

```powershell
python ".agents/skills/draw-iphone/scripts/generate_iphone_photo_prompts.py" --manifest ".\iphone-photo-prompts.jsonl" --image2-out ".\draw-iphone-run"
```

Default image2 settings:

- Model: `gpt-image-2`
- Quality: `medium`
- Requested image2 size: `2048x944`
- Final normalized asset size: `2040x936`
- Frontend hand-card reference: `340x156`, so final assets are exactly 6x the current hand-card ratio.
- Base URL: `https://xaapi.ai/v1`
- Auth: `OPENAI_API_KEY` from process, Windows User env, or Windows Machine env
- Output format: `png`
- Concurrency: `2`
- Raw photo output: `images/`
- Deterministic game-frame preview output: `framed/`

Do not put API keys in files or prompts.

The image2 provider may return inconsistent natural dimensions. Always keep local normalization enabled unless diagnosing provider output. Use `--no-normalize-images` only for debugging.

Validate a production manifest:

```powershell
python ".agents/skills/draw-iphone/scripts/generate_iphone_photo_prompts.py" --validate-manifest ".\iphone-photo-prompts.jsonl"
```

Print the finished-image QA checklist:

```powershell
python ".agents/skills/draw-iphone/scripts/generate_iphone_photo_prompts.py" --print-image-qa
```

## Production Contract

Every production row must include:

- `schema_version`: prompt manifest format version.
- `style_signature`: fixed style contract id, currently `CF250_IPHONE_REALPHOTO_V1`.
- `index`: stable batch order.
- `item_name`: original item name.
- `subject_kind`: `physical-item` or `tangible-token`.
- `subject`: concrete photographed subject.
- `frontend_card_width`: current hand-card width, `340`.
- `frontend_card_height`: current hand-card height, `156`.
- `asset_width`: final normalized asset width, `2040`.
- `asset_height`: final normalized asset height, `936`.
- `asset_aspect_ratio`: `340:156`.
- `photo_role`: `raw_item_art`.
- `ui_overlay_required`: `true`.
- `visual_grade`: `bronze`, `silver`, `gold`, `diamond`, or `legendary`.
- `frame_style`: deterministic frame style derived from `visual_grade`.
- `prompt`: final image prompt to send to the generator.
- `qa_status`: prompt validation state.

Do not place the style signature inside the image prompt. It is metadata for batch control, not visible image content.

Do not ask image2 to draw rarity borders, card frames, title bars, prices, stats, or badges. Keep those in deterministic overlay metadata and the local `framed/` preview renderer.

## Quality Rules

- One prompt equals one image.
- One input item must become one separate prompt row; never combine several item names into one image prompt.
- One image has one dominant item.
- Final image files must be `2040x936` unless the frontend card contract changes.
- Raw photo files in `images/` must not contain card frames or text.
- Framed previews in `framed/` may contain deterministic borders, dark title/price shelves, corner plates, and rarity pips.
- Use ordinary real-life surfaces and natural lighting.
- Prefer documentary smartphone realism over perfect studio product photography.
- For services or events, photograph one tangible token: receipt, bill, ticket, invoice, form, phone order screen, or deposit slip.
- Preserve brand-bearing item names in `item_name`, but keep logos non-dominant in the prompt.
- Avoid readable real personal data, celebrity references, extra objects competing with the subject, filters, artificial effects, and rendered-art wording.
- Reject prompts that omit the fixed iPhone base or contain banned old-style words.
