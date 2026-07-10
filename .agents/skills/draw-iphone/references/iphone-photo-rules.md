# Item Art Rules

## Contents

- Production contract, master art direction, and six treatments
- Stable mixed assignment
- Semantic categories and visual archetypes
- Service conversion and composition
- Size, manifest, prompt, and image QA
- Legacy compatibility

## Production Contract

Use item-art-prompt-manifest/v2 for new batches. New prompts use these signatures:

- art direction: CF250_GAME_ITEM_ART_BIBLE_V1
- style set: CF250_ITEM_ART_COHESIVE_V3

- CF250_ITEM_ART_COHESIVE_V3/fauvist-paint
- CF250_ITEM_ART_COHESIVE_V3/pop-art-print
- CF250_ITEM_ART_COHESIVE_V3/constructivist-poster
- CF250_ITEM_ART_COHESIVE_V3/baroque-still-life
- CF250_ITEM_ART_COHESIVE_V3/hyperreal-ad
- CF250_ITEM_ART_COHESIVE_V3/retro-east-asian-ad

Keep the signature in manifest metadata, not visible image content.

## Master Art Direction

Every new mixed-style prompt must keep:

- one product as the only hero;
- dynamic three-quarter or strong diagonal orientation;
- complete readable silhouette at 340x156;
- cinematic commercial realism with a polished hand-painted finish;
- localized key and rim light;
- readable midtones and open shadow detail;
- two broad background color masses and one shallow environmental plane;
- product-authentic body color, construction, controls, ports, lids, handles, and materials.

Do not use six independent visual worlds. The master art direction owns geometry, lighting, scale, silhouette, and background complexity. A profile may change only brushwork, print texture, surface finish, and secondary color treatment.

## Six Surface Treatments

| Profile | Visual mechanism | Main risk |
| --- | --- | --- |
| fauvist-paint | Broad background brushwork and reflected complementary color | Keep brushwork off silhouette-critical edges |
| pop-art-print | Selective halftone, silkscreen texture, restrained contour | Keep the product dimensional rather than comic-flat |
| constructivist-poster | Two or three diagonal background planes, screen-print texture | No poster text and no red-black template repetition |
| baroque-still-life | Oil glazing, jewel depth, one warm beam | Raise shadows; never crush the product into black |
| hyperreal-ad | Physical set, three-quarter close-up, hard flash | Retain natural materials and avoid sterile CGI gloss |
| retro-east-asian-ad | Airbrush-photo hybrid, broad color fields, print grain | Avoid generic nostalgia props and fake packaging text |

Treat these as subordinate production treatments, not free-form style names.

## Palette Profiles

Assign one stable palette independently from the style profile:

- ember-cobalt
- tropical-daylight
- candy-electric
- jade-gold
- citrus-steel
- plum-mint

Each palette has two dominant background hues and one restrained accent. Preserve the product's authentic body color. This prevents one style from repeating the same red-black or yellow-blue background across unrelated goods.

## Stable Mixed Assignment

Default style_profile is auto.

Compute SHA-256 over UTF-8(style_seed + NUL + item_name) for style assignment. Use a separate palette-prefixed digest for palette assignment. Do not use process randomness or Python hash().

The default seed is cf250-item-art-v1. Keep it fixed for the production asset set.

The same item and seed must keep the same profile after:

- retrying one failed image;
- changing input order;
- splitting or merging batches;
- running on another machine.

Change the seed only when intentionally producing a new art-direction pass.

Tickets, bills, receipts, and service tokens use this safer automatic subset:

- pop-art-print
- constructivist-poster
- hyperreal-ad
- retro-east-asian-ad

Fauvist and Baroque remain available as explicit overrides for experiments, but are excluded automatically because they more often deform document structure.

## Universal Subject Rules

- One item name becomes one prompt, one job, and one image.
- Keep exactly one dominant item with no duplicate copy.
- Preserve identity, proportions, function, silhouette, and defining materials.
- Keep the chosen treatment visible but subordinate to product identity.
- Keep the subject readable when reduced to 340x156.
- Keep a narrow 6 percent edge margin for deterministic UI overlay.
- Direct the brightest local contrast toward the product, not the background.
- Keep shadow detail open; dark treatments must still pass thumbnail review.
- Do not add people, hands, competing props, card frames, UI, rarity badges, title bars, stats, signatures, or watermarks.
- Do not add readable personal data or invented readable typography.
- Keep unavoidable package or document printing abstract and unreadable.
- Do not restore satire, floating money, warning neon, or dark-humor framing.

## Semantic Categories

Classify each item before building the prompt:

- food
- drink
- drinkware
- personal-care
- stationery
- jewelry
- electronics
- appliance
- cookware
- apparel
- ticket
- bill-or-receipt
- service-token
- general-product

Use category language only to preserve physical plausibility:

- food: edible structure and fresh surface texture, no garnish;
- drink: real container and plausible liquid, glass, condensation, foam, or translucency;
- drinkware: authentic vessel, lid, handle, straw, base, finish, and scale;
- personal-care: packaging proportions and glass, cream, soap, wax, or plastic;
- stationery: functional geometry, nib, barrel, paper, wood, or metal;
- jewelry: controlled reflections, scale, joints, settings, and fine metal detail;
- electronics: exact geometry, controls, ports, screen, glass, metal, plastic, wear, or damage;
- appliance: authentic footprint, controls, vents, integral cable, sensors, and moving parts where present;
- cookware: authentic body, lid, rim, handles, enamel, metal, ceramic, and manufacturing details;
- apparel: complete wearable silhouette, fabric, stitching, leather, or rubber;
- ticket: one flat ticket or pass with unreadable printing;
- bill-or-receipt: one flat bill, invoice, or receipt with no readable private data;
- service-token: one tangible receipt, ticket, form, damaged object, or payment token;
- general-product: real proportions, function, silhouette, and material.

Classify appliance before drink. `Nespresso 胶囊机` is an appliance, not a drink. Classify drinkware before drink. `Stanley Quencher 保温杯` is drinkware, not a generic product.

## Visual Archetypes

Use a second field for physical composition:

- food-closeup
- tall-vessel
- handheld-electronics
- appliance
- cookware
- soft-apparel
- flat-document
- small-hard-good
- hero-product

Semantic category preserves plausibility. Visual archetype controls scale, angle, silhouette, and motion cue. Do not make one keyword list serve both jobs.

## Service Conversion

Represent non-physical purchases with one tangible token:

- ticket or event -> one paper ticket or one phone ticket screen;
- bill or fee -> one receipt, invoice, payment slip, or phone payment page;
- repair or replacement -> one damaged object or one repair invoice;
- medical expense -> one registration slip, deposit bill, medicine box, or appointment form;
- travel expense -> one boarding pass, hotel bill, fuel receipt, or ride receipt.

Never combine the service, receipt, phone, money, and surrounding scene in one image.

## Composition by Archetype

- Flat ticket or bill: full outline visible, slight perspective angle, 70 to 82 percent of frame width.
- Tall vessel or handheld device: long axis placed diagonally, 76 to 88 percent of the usable long dimension.
- Appliance: grounded three-quarter hero view with its complete footprint and no invented attachment.
- Cookware: low three-quarter view separating body, lid, rim, handles, and finish.
- Apparel: complete wearable silhouette, 68 to 80 percent of the usable frame, with one controlled fold or lifted edge.
- Food, small hard goods, and general products: 78 to 86 percent of usable area with the identifying silhouette visible.

Do not force a tall bottle into a centered vertical catalog pose inside the wide card.

## Size Contract

The frontend hand-card reference is 340x156.

Request 2048x944 from gpt-image-2 and normalize locally to 2040x936.

Raw art belongs in images/. Thin deterministic frame previews belong in framed/. True-size batch contact sheets belong in review/.

Do not ask the image model to draw the frame.

## V2 Manifest Fields

Every new row includes:

- schema_version
- style_signature
- style_profile
- style_family
- style_assignment
- style_seed
- art_direction_signature
- visual_archetype
- palette_profile
- index
- item_name
- subject_kind
- subject_category
- subject
- frontend_card_width and frontend_card_height
- asset_width, asset_height, and asset_aspect_ratio
- photo_role and ui_overlay_required
- visual_grade and frame_style
- prompt
- qa_status

## Prompt QA

Pass only prompts that contain:

- one single-item subject;
- wide horizontal 340:156 item-card composition;
- one dominant item only;
- category-safe scale and orientation;
- narrow 6 percent safe margin;
- one exact style-profile anchor;
- the master art-direction anchor;
- one stable palette profile;
- product-identity preservation;
- no duplicate, UI, border, private data, signature, or watermark.

## Image QA

Pass only images that satisfy all checks:

- one recognizable item and no duplicate;
- authentic product geometry, hardware layout, materials, and defining parts;
- visible treatment subordinate to the product;
- two broad background masses, one restrained accent, readable midtones, and open shadows;
- readable silhouette and focal contrast at 340x156;
- no competing props, people, hands, frame, or UI;
- no readable private data, invented typography, signature, or watermark;
- exact final size 2040x936.
- reviewed beside adjacent items in review/contact-*.png.

Reject and regenerate failures row by row.

Text-only generation cannot guarantee an exact branded model. When model fidelity is required, use a product reference image. Without one, reject invented ports, controls, logos, openings, sensors, or accessories during image QA.

## Legacy Compatibility

Use --style-profile iphone-realphoto to emit the original:

- schema: iphone-product-photo-prompt-manifest/v1
- signature: CF250_IPHONE_REALPHOTO_V1
- fixed no-filter iPhone prompt

Existing V1 manifests remain valid and can still be passed to --manifest for generation, validation, normalization, and frame rendering.

Existing `CF250_ITEM_ART_MIXED_V2/*` manifests also remain valid. Validation accepts their old signatures and prompt anchors; new generation never emits them.
