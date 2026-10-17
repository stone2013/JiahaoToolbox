# Independent final review — SMV6 v3

Reviewed 2026-10-17. Scope: custom single brand shot, native 1080p, not a Gallery reproduction. Inputs inspected: final MP4 extracted frames f000/020/045/070/100/150/179 and contact sheet, src/index.tsx, DESIGN_SPEC.md, synthesis/render/QA scripts, ffprobe.json, audio-analysis.txt, final-review.md, aesthetic-rules.md, logo-shrink-wordmark-lockup card and exact demo TSX. Composition not modified.

## Final disposition

PASS with limitations below. No blocking issue remains in inspected final evidence. Rechecked refreshed final f150 at1920×1080: tagline CSS45px now measures32px bright glyph height (y665–696), meeting Q11 auxiliary threshold. This replaces the earlier25px/18px finding.

## Brief and final-review checks

T1 ✓ ffprobe final: H.264 High/yuv420p,1920×1080,60/1fps,180 decoded frames,3.000000s; AAC stereo48kHz3.000000s. Composition itself is1920×1080 and uses native CSS typography, no upscaled video/image input.
P1/P2/P3 ✓ f100: SMV6 brand and exact requested tagline, no unrequested feature claims. P4 ✓ brief choices map to timeline/color/audio in spec, although decision table is prose rather than tabular.
F1 ✓ wordmark f020, blue line f045, tagline f070 and full lockup f100. F2 ✓ one shot, no repetition. F3 N/A no product interface.
V1/V2/V3/V4 ✓ f000–179: black/white with restrained blue line, no particles, rotation, strong glow, overshoot or extraneous style. Cubic-bezier(.22,1,.36,1) and title scale.955→1 implement smooth restrained settling.
S1/S5 ✓ reference lockup/late tagline intent applicable to custom shot. S2/S3/S4/S6 N/A variant reproduction not claimed; exact demo has neon/rotation/overshoot/4.4s and is intentionally not copied, as spec records. No Gallery video match certified.
B1/B2 ✓ code and extracted frames align with documented timing. B3 ✓ wordmark static f53–179 (2.12s); all elements settled f99–179 (1.35s). B4 unverifiable production-history approval trail, no issue evident in delivered spec.
D1/D2 ✓ text-only brand, no sensitive data. D3/D4/D5 N/A no screenshots, dynamic data or private source imagery.
A1/A6 limited: output audio exists, measured mean−31.5dB/max−21.1dB, ample headroom without clipping. Quietness suits stated soft direction technically, but subjective loudness/timbre/listening not verified. A2 ✓ intended starts.08/.52/1.02s correspond approximately title/line/tagline beginnings, soft envelope deliberately broad. A3/A4/A5/A7/A8/A9 N/A no beat track, UI audio, riser-impact sequence, long library sounds or BGM. Exact output audio-offset correlation not performed; no hard transient claims.

## Aesthetic rules — all entries

R1 ✓ f53–179 brand hold and f99–179 full lockup hold. R2 ✓ nonlinear easing; no batch motion. R3 exception: whole user-requested clip is3s, entrance arc ends1.65s; a≥3s moving arc cannot also retain1s hold. User duration overrides generic rule. R4 ✓ no camera/beat impacts.
Q1 ✓ custom abstract brand graphic, no page recreation. Q2 N/A no3D raster texture; fullsize f100 typography sharp. Q3 ✓ fixed camera; source deterministic and no jitter expressions. Q4 ✓ no glint/glow. Q5 ✓ SMV6 is sole hero. Q6 ✓ front-facing readable wordmark. Q7/Q8 N/A no object3D/multifunction product finale; user prohibits rotation/particles. Q9/Q10 N/A no UI landing slots/document mock. Q11 ✓ refreshed final f150 tagline32px effective height (measured y665–696); meets auxiliary threshold.
S1 exception: user explicitly requested soft electronic sound; original layered tones fulfill stated direction by construction, strong beat BGM generic recommendation does not apply. Subjective hearing not certified. S2 ✓ deterministic source starts documented in synthesis; no repeats. S3 limited: final timeline/audio starts inspected, production chronology unverified. S4 N/A no physical interaction needing foley; sound fades by2.9s. S5 limited: no measured output-offset compensation and no sharp sync claim.
C1 ✓ clean brand shot exempt from narration. C2 user-supplied tagline retained verbatim. C3 N/A no3D.
P1 ✓ actual extracted MP4 frames inspected fullsize and contact sheet. P2 ✓ exact reference card/demo inspected, custom scope documented. P3 N/A no ambiguous feedback in review scope. P4 ✓ three required brand elements map to one shot, no duplicate technique.

## Recommendations and limitations

Recommended: verify quiet electronic cue by actual playback at ordinary listening volume. No subjective listening was performed by this reviewer. Rendered H.264 frames have ordinary small compression differences during hold; source transforms are clamped/static. No animated reference Gallery sample supplied/inspected, and no reproduction equivalence is claimed. Independent rerender/determinism comparison and all180 visual frames were not exhaustively inspected. Editable TSX, locked package dependencies, synthesis source and render command are included.

Disposition: PASS with stated audio/reference/exhaustive-frame limitations. No technical format defect, remaining blocking issue or prohibited visual effect found in inspected evidence.
