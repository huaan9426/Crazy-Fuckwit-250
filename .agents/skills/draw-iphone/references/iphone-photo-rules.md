# iPhone Photo Rules

## Style Signature

Use this metadata value for every production row:

```text
CF250_IPHONE_REALPHOTO_V1
```

The signature means:

- rear-camera iPhone snapshot;
- natural light;
- no filter;
- no stylization;
- documentary everyday realism;
- one dominant item;
- wide horizontal card asset;
- safe margin for deterministic UI overlay;
- no decorative game-card language.

Do not include the signature text in the image prompt.

## Fixed Base

Append this base to every prompt:

```text
Photorealistic image captured on iPhone 17 Pro Max rear camera, f/1.78 aperture, natural lighting, extremely detailed, sharp focus, authentic smartphone snapshot, zero filters, zero stylization, true-to-life colors, subtle lens flare if applicable, natural skin texture, real iPhone photography, 4K resolution feel, documentary realism, casual everyday photo
```

## Prompt Shape

Use this shape:

```text
Close-up iPhone snapshot of a single [real-world subject], [plain everyday context], landscape horizontal frame for a wide marketplace item tile art crop, one dominant item only, no dominant logo, no readable private information, [fixed base]
```

Use this stricter generated prompt shape:

```text
Close-up iPhone snapshot of a single [real-world subject], landscape horizontal frame for a wide marketplace item tile art crop, centered product silhouette, safe empty margin near all edges for deterministic UI overlay, one dominant item only, casual handheld composition, no text, no border, no decorative frame, no dominant logo, no extra props competing with the subject, [fixed base]
```

## Frontend Size Contract

The current frontend hand cards are drawn in `src/game/CheckoutRushScene.ts` with a runtime maximum of `340x156`. Generate and store source assets at `2040x936`, exactly 6x that ratio.

Use image2 request size `2048x944` because gpt-image-2 requires dimensions divisible by 16. After generation, locally normalize the final files to `2040x936`.

The detail flip card is currently `320..460 x 210..270`, so the hand-card asset may be cropped or fitted by future UI work. Do not change this skill's asset size unless the frontend card contract changes.

## Subject Conversion

For physical goods, photograph the item itself.

For services, accidents, tickets, bills, fees, medical costs, travel costs, or digital purchases, photograph one tangible token that represents the item:

- ticket item -> one paper ticket or phone ticket screen
- bill item -> one receipt, invoice, payment slip, or phone payment page
- repair item -> one damaged object or one repair invoice
- medical item -> one registration slip, deposit bill, medicine box, or appointment form
- travel item -> one boarding pass, hotel bill, fuel receipt, or ride receipt

## Negative Discipline

Keep the prompt in documentary smartphone-photo language only. Do not add game-card decoration, artificial dramatic effects, floating decorative objects, rendered-art framing, or joke/satire framing.

Do not ask image2 to draw rarity borders, card frames, title bars, stat boxes, price tags, cooldown badges, or text. Those are deterministic UI overlays.

## Game UI Layer

Keep two asset layers:

- `images/`: raw normalized photo art, no frame, no text, no UI.
- `framed/`: deterministic preview frames rendered by the local script.

Visual grades are metadata, not prompt style:

- `bronze` -> aged bronze frame
- `silver` -> brushed silver frame
- `gold` -> warm gold frame
- `diamond` -> cool prism frame
- `legendary` -> red-gold legendary frame

The local frame renderer may add borders, dark title/price shelves, corner plates, and rarity pips. These must be identical for the same `visual_grade`.

## Prompt QA

Each prompt must contain:

- `Close-up iPhone snapshot`
- `single`
- `landscape horizontal frame for a wide marketplace item tile art crop`
- `centered product silhouette`
- `safe empty margin near all edges for deterministic UI overlay`
- `one dominant item only`
- `casual handheld composition`
- `no text`
- `no border`
- `no decorative frame`
- `no dominant logo`
- `no extra props competing with the subject`
- `Photorealistic image captured on iPhone 17 Pro Max rear camera`
- `zero filters`
- `zero stylization`
- `documentary realism`

Reject any prompt containing old visual direction terms such as artificial card styling, satire styling, floating money, warning neon, concept art, poster art, rendered art, or illustration language.

For brand-bearing goods, preserve the provided item name in manifest metadata, but do not make a logo, slogan, or label layout the main subject of the image.

## Image QA

For finished images, pass only images that satisfy all of these checks:

- one clear main subject;
- looks like a phone photo, not studio product render or illustration;
- natural lighting and true-to-life color;
- no decorative border, poster layout, card frame, floating props, or visual effects;
- no readable private information on tickets, receipts, bills, or screens;
- item fills most of the frame but still looks casually photographed;
- no brand logo dominating the image;
- no text/signature/watermark added by the model.
- exact final pixel size `2040x936`.

## Batch QA

For a production batch:

- item count equals prompt row count;
- every row has `style_signature=CF250_IPHONE_REALPHOTO_V1`;
- every row has `photo_role=raw_item_art`;
- every row has `ui_overlay_required=true`;
- every row has `visual_grade` and matching `frame_style`;
- every row has one prompt only;
- no prompt contains multiple input item names unless the item itself is a compound product name;
- indexes are unique and stable;
- every final image in `images/` is `2040x936`;
- if `framed/` is produced, every framed preview is `2040x936`;
- failures are regenerated row by row, not by merging failed items into one prompt.
