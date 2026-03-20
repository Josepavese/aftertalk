# Social Preview Image — Spec

**Dimensions**: 1280 × 640 px (required by GitHub)
**File**: `docs/assets/social-preview.png` (then upload manually)
**Upload**: GitHub → Settings → Social preview → Edit → upload file

This image appears when the repo link is shared on Twitter/X, LinkedIn,
Slack, iMessage, etc. It is the single most visible marketing asset.

---

## Design

```
┌────────────────────────────────────────────────────────────────────┐
│  bg: #0f172a (dark slate)                                          │
│                                                                    │
│   ◆  Aftertalk          [logo/wordmark, large, white+indigo]       │
│                                                                    │
│   WebRTC · AI Transcription · Session Minutes                      │
│   [subtitle, small, #94a3b8 slate-400]                             │
│                                                                    │
│   ┌──────────────┐  ┌──────────────┐  ┌──────────────┐            │
│   │  🎙 Record   │→ │  📝 Transcri │→ │  📄 Minutes  │            │
│   │  WebRTC      │  │  STT + LLM   │  │  via Webhook │            │
│   └──────────────┘  └──────────────┘  └──────────────┘            │
│   [3 cards with indigo border, icon + 2 lines each]                │
│                                                                    │
│   Privacy-first · Self-hosted · No audio stored                    │
│   [footer tag, slate-500, small]                                   │
│                                                                    │
│                           github.com/Josepavese/aftertalk          │
└────────────────────────────────────────────────────────────────────┘
```

## Palette

| Role | Color | Hex |
|------|-------|-----|
| Background | Dark slate | `#0f172a` |
| Title | White | `#f8fafc` |
| Accent / arrows | Indigo | `#6366f1` |
| Subtitle | Slate 400 | `#94a3b8` |
| Card border | Indigo 30% | `#6366f150` |
| Card bg | Slate 800 | `#1e293b` |
| Footer | Slate 500 | `#64748b` |

## Tools

Any of these work:
- **Figma** (recommended) — precise control, export @2x
- **Canva** — use "Custom size 1280×640"
- **OG Image generators**: og-image.vercel.app, imgsrc.io

## How to upload (manual step — GitHub API does not support this)

```
1. Create image → save as social-preview.png
2. GitHub → Josepavese/aftertalk → Settings (gear icon)
3. Scroll to "Social preview" section
4. Click "Edit" → upload social-preview.png
5. Save
```

Verify at: https://www.opengraph.xyz/url/https%3A%2F%2Fgithub.com%2FJosepavese%2Faftertalk
(works only when repo is public)
