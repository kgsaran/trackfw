# Project Instructions

<!-- trackfw:rules:start -->
## trackfw — Governance Rules

This project uses **trackfw** for AI-native delivery governance.
Chain: `ADR → REQ → ROADMAP` · States: `backlog / analyzing / wip / blocked / done / abandoned`

### Agent Protocol
1. **Before any implementation (mandatory):** create governance artifacts FIRST, then branch:
   `trackfw req new "title"` → `trackfw roadmap new "title"` → `trackfw roadmap move <name> wip` → `git checkout -b feat/<branch>`
   ❌ Never create a branch before REQ + ROADMAP are in wip/
   ❌ Never defer REQ/ROADMAP creation to a future task — they are prerequisites, not deliverables
   ✓ `trackfw validate` enforces this via `branch_has_wip_roadmap` rule (v2.7.0+)
2. **Before starting:** run `trackfw context` · read `docs/agents-working-context.md`
3. **After finishing:** update `docs/agents-working-context.md` with what changed
4. **Before PR:** `trackfw validate` must pass
5. **ML lifecycle — mandatory:**
   - Starting a ML: edit roadmap `**Status:** ⬜ Pendente` → `**Status:** 🔄 Em andamento` + commit.
   - Completing a ML: edit roadmap → `**Status:** ✅ Concluído` + include in ML commit.
   - Analyzing a roadmap: move from `backlog/` to `analyzing/`; to `wip/` only when coding starts.
6. **Obrigatório: Inspecione e respeite todos os ADRs globais nos diretórios listados em adr_dirs (inclusive caminhos ~/...) antes de propor alterações de arquitetura.**

### Attention Signal (when you need user input during a task)
Write `docs/roadmaps/.trackfw-attention.json`:
```json
{"roadmap":"file.md","ml":"ML-1A","message":"what you need","level":"action_required","timestamp":"ISO8601Z"}
```
Delete the file when resolved. Visible as a live banner in `trackfw serve`.

> **Windsurf users:** before asking the user a question or requesting approval, write
> `<roadmap_dir>/.trackfw-attention.json` manually — there is no automatic hook for this.
> Delete the file after the user responds.

### Architecture Directives (mandatory)
- **3-layer separation:** frontend / backend / database — never mix concerns
- **No in-memory data:** always database + ORM (never arrays/globals for persistence)
- **Auth from day 1:** never defer — refactoring auth later is very costly
- **Docker + .env from day 1:** containerize early; all config via env vars
- **2-layer validation:** frontend (UX) + backend (security) — never only one
- **API-first:** define OpenAPI contract before coding frontend/backend integration
- **Security wave:** include a red-team review wave in every feature roadmap
- **Test coverage:** TDD for critical logic; min 60% (prototype) / 80% (production)
- Use `/trackfw:architect` to define stack before the first REQ

### Key Commands
- `trackfw context` — current governance state (always run first)
- `trackfw status` — all artifacts and states
- `trackfw validate` — governance consistency check
- `trackfw roadmap move <name> <state>` — transition roadmap state
- `trackfw serve` — live Kanban board at http://localhost:4080
<!-- trackfw:rules:end -->
