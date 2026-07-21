# Gemini Notebook (NotebookLM) — UI Feature Map

**Date:** 2026-07-21  
**Method:** Chrome DevTools MCP accessibility snapshots on logged-in session  
**Product URL:** https://notebooklm.google.com/  
**Rebrand note:** UI displays "Gemini Notebook"; URLs remain `notebooklm.google.com`.

---

## 1. Home (`/`)

### Navigation & filters
| Control | Type | Options / behavior |
|---------|------|------------------|
| Filter tabs | Radio group | **All**, **My notebooks** (default), **Featured notebooks** |
| Search | Button → search UI | "Open search" — title/content search across notebooks |
| View mode | Radio group | **Grid view** (default), **List view** |
| Sort | Menu button | **Most recent** (default); menu also exposes **Title** |
| Create | Button | "Create new notebook" (header + section CTA) |
| Settings | Menu | Help, Feedback, Discord, Output Language, Licenses, Theme (Device/Light/Dark), Upgrade |

### Notebook cards (My notebooks)
Each card shows: emoji icon, title, date, source count.  
**Project Actions Menu** (per card):
- Delete
- Edit title
- Pin to top

### Featured notebooks
Public notebooks curated by partners (Google Research, OpenStax, Arts & Culture, etc.).  
Cards show publisher name, title, date, source count. Description: "This project is public and anyone with link can view".  
No project actions menu on featured cards (read-only templates).

---

## 2. Notebook page (`/notebook/{uuid}`)

Three-panel layout: **Sources** (left) | **Chat** (center) | **Studio** (right).  
Panels are collapsible.

### Top bar
| Control | Behavior |
|---------|----------|
| Editable title | Inline textbox; auto-saves |
| Create notebook | Opens new empty notebook |
| Analytics | Expandable panel (usage stats) |
| Share notebook | Sharing dialog |
| Settings | Same menu as home |

### Share dialog
- Add people (PeopleStack autocomplete)
- Notify people toggle
- Role selector (Owner for creator)
- Notebook Access: **Restricted** (default) / link sharing
- Copy link
- More copy options

---

## 3. Sources panel

| Control | Behavior |
|---------|----------|
| Add source | Modal with source type picker |
| Discover sources | Query textbox + submit |
| Discover scope menu | **Web** ("Best sources from the web"), **Drive** ("Your content from Google Drive"; auto-synced after import) |
| Research mode menu | **Fast Research** (quick), **Deep Research** (in-depth) |
| Auto-label | "Auto-label your sources by topic" |
| Sort sources | Recent, Title, Type |
| Select all | Checkbox toggles all sources for chat scope |
| Per-source row | Title button (opens source), checkbox (include in chat), More menu |

**Per-source More menu:**
- Remove source
- Rename source

**Add source modal types:**
- Upload files (PDF, images, docs, audio, and more; drag-and-drop)
- Websites (URL)
- Google Drive
- Copied text

Sources can carry topic labels (visible as `[D]` prefix on discovered sources).

---

## 4. Chat panel

| Control | Behavior |
|---------|----------|
| Configure notebook | Opens "Configure Chat" dialog |
| Chat options menu | Customize notebook; Delete chat history (disabled when empty) |
| Customize notebook | Persona/emoji/title customization |
| Auto-summary | Generated on notebook open from selected sources |
| Summary actions | Save to note, Copy, Good/Bad feedback |
| Suggested prompts | Contextual starter questions (3 buttons) |
| Query box | Multiline input; shows active source count; Submit |

**Configure Chat dialog:**
- **Conversational goal:** Default | Learning Guide | Custom
- **Response length:** Default | Verbose | Concise
- Save settings button

Chat history persists across sessions (per Chat options tooltip).

---

## 5. Studio panel

### Generation tiles (click opens customize dialog or one-click generate)
| Artifact | Badge | Customize options observed |
|----------|-------|---------------------------|
| Audio Overview | — | Format: Deep Dive, Brief, Critique, Debate; Language combobox; Length: Short/Default/Long; Focus prompt |
| Slide Deck | BETA | Format: Detailed Deck, Presenter Slides; Language; Length: Short/Default; Description prompt |
| Video Overview | — | Format: Explainer, Short (new); Language; Visual style carousel (Auto-select, Custom, Classic, Whiteboard, Kawaii, Anime, Watercolor, Retro print, Heritage, Paper-craft); Focus prompt + suggestions |
| Mind Map | — | Customize Mind Map sub-button |
| Reports | — | (Dialog not fully captured; one-click or customize) |
| Flashcards | — | (Dialog not fully captured) |
| Quiz | — | (Dialog not fully captured) |
| Infographic | BETA | (Dialog not fully captured) |
| Data Table | — | (Dialog not fully captured) |
| Add note | — | Manual note creation |

### Multi-language audio banner
"Create an Audio Overview in: हिन्दी, বাংলা, ગુજરાતી, …" (Indian languages quick links).

### Short Video Overviews promo
Banner: "Try new Short Video Overviews!" with Try it / Dismiss.

### Generated artifacts list
Each artifact card shows: title, duration, type (Explainer), source count, age.  
Actions: **Play**, **More** menu.

**Artifact More menu:**
- Share
- Rename
- Download
- View prompt and sources
- Delete

---

## 6. Settings (global)

Available on home and notebook:
- Gemini Notebook Help
- Send feedback
- Discord community link
- Output Language
- Licenses
- Theme (Device / Light / Dark submenu)
- Upgrade Gemini Notebook (subscription upsell)

---

## 7. Identity & URLs

- Notebook ID: UUID in path (`/notebook/{uuid}`)
- Account: Google Account session (Chrome cookies)
- No official public REST API; all mutations via internal `batchexecute` RPC

---

## 8. CLI parity mapping (notebooklm-py)

| UI surface | CLI command group |
|------------|-------------------|
| Home list/create/delete/rename/pin | `notebook list/create/delete/rename` |
| Sources add/discover/labels | `source add/list/delete/refresh`, `research`, `label` |
| Chat query/configure/history | `chat ask/history/configure` |
| Studio generate/download | `generate`, `download`, `artifact` |
| Share | `share` |
| Notes | `note` |
| Local cache | `sync`, `search` |
| Auth | `auth login --chrome`, `doctor` |
