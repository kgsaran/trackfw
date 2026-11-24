---
layout: doc
aside: false
title: "trackfw — Delivery governance for AI agents and teams"
---

<style>
.hero-section {
  text-align: center;
  padding: 4rem 1rem 3rem;
  border-bottom: 1px solid var(--vp-c-divider);
}
.hero-section h1 {
  font-size: 2.4rem;
  font-weight: 700;
  line-height: 1.2;
  margin-bottom: 1rem;
}
.hero-actions {
  display: flex;
  gap: 1rem;
  justify-content: center;
  flex-wrap: wrap;
  margin-top: 1.5rem;
}
.btn-brand {
  display: inline-block;
  padding: 0.6rem 1.4rem;
  background: var(--vp-c-brand-1);
  color: #fff !important;
  border-radius: 6px;
  font-weight: 600;
  text-decoration: none !important;
}
.btn-alt {
  display: inline-block;
  padding: 0.6rem 1.4rem;
  border: 1px solid var(--vp-c-divider);
  border-radius: 6px;
  font-weight: 600;
  text-decoration: none !important;
}
.section {
  max-width: 900px;
  margin: 0 auto;
  padding: 3rem 1.5rem;
}
.section-title {
  font-size: 1.5rem;
  font-weight: 700;
  margin-bottom: 1.5rem;
}
.problem-block {
  font-size: 1.05rem;
  line-height: 1.8;
  color: var(--vp-c-text-2);
  border-left: 3px solid var(--vp-c-brand-1);
  padding-left: 1.25rem;
}
/* Chain diagram — 2×3 grid on desktop, single column on mobile */
.chain-diagram {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
}
.chain-row {
  display: flex;
  align-items: stretch;
  gap: 0.3rem;
  flex-wrap: nowrap;
}
.chain-node {
  background: var(--vp-c-bg-soft);
  border: 1px solid var(--vp-c-divider);
  border-radius: 6px;
  padding: 0.5rem 0.9rem;
  font-size: 0.88rem;
  flex: 1;
  min-width: 0;
}
.chain-node code {
  font-size: 0.8rem;
  background: transparent;
  color: var(--vp-c-brand-1);
  white-space: nowrap;
}
.chain-arrow {
  display: flex;
  align-items: center;
  color: var(--vp-c-brand-1);
  font-weight: 700;
  font-size: 1rem;
  flex-shrink: 0;
  padding: 0 0.2rem;
}
.chain-down {
  color: var(--vp-c-brand-1);
  font-weight: 700;
  font-size: 1rem;
  padding-left: calc((100% / 3 - 1rem) / 2 + 0.5rem);
}
@media (max-width: 640px) {
  .chain-row {
    flex-direction: column;
    flex-wrap: wrap;
  }
  .chain-arrow {
    transform: rotate(90deg);
    justify-content: flex-start;
    padding-left: 0.5rem;
  }
  .chain-down {
    padding-left: 0.5rem;
  }
}
.chain-learn {
  margin-top: 1rem;
  font-size: 0.9rem;
}
.scenarios-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 1.5rem;
  margin-top: 1rem;
}
@media (max-width: 768px) {
  .scenarios-grid {
    grid-template-columns: 1fr;
  }
}
.scenario-card {
  border: 1px solid var(--vp-c-divider);
  border-radius: 8px;
  padding: 1.25rem;
  background: var(--vp-c-bg-soft);
}
.scenario-card h3 {
  font-size: 1rem;
  font-weight: 700;
  margin-bottom: 0.5rem;
}
.scenario-card p {
  font-size: 0.9rem;
  color: var(--vp-c-text-2);
  line-height: 1.6;
  margin-bottom: 0.75rem;
}
.scenario-link {
  font-size: 0.85rem;
  font-weight: 600;
  color: var(--vp-c-brand-1);
  text-decoration: none;
}
.board-section img {
  border-radius: 8px;
  box-shadow: 0 2px 12px rgba(0,0,0,0.12);
  max-width: 100%;
}
.demo-section video {
  border-radius: 8px;
  box-shadow: 0 2px 12px rgba(0,0,0,0.12);
  max-width: 100%;
}
.features-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 1.5rem;
  margin-top: 1rem;
}
.feature-card {
  padding: 1rem;
  border: 1px solid var(--vp-c-divider);
  border-radius: 8px;
  background: var(--vp-c-bg-soft);
}
.feature-card .icon {
  font-size: 1.5rem;
  margin-bottom: 0.5rem;
}
.feature-card h3 {
  font-size: 0.95rem;
  font-weight: 700;
  margin-bottom: 0.4rem;
}
.feature-card p {
  font-size: 0.85rem;
  color: var(--vp-c-text-2);
  line-height: 1.6;
}
.section-divider {
  border: none;
  border-top: 1px solid var(--vp-c-divider);
  margin: 0;
}
.terminal-img {
  border-radius: 8px;
  box-shadow: 0 2px 12px rgba(0,0,0,0.12);
  max-width: 100%;
  display: block;
  margin-top: 1rem;
}
</style>

<!-- ============================================================
  HERO
  ============================================================ -->
<div class="hero-section">
  <h1>Your AI coding agent ships fast.<br>trackfw makes it ship traceably.</h1>
  <p style="font-size:1.1rem;color:var(--vp-c-text-2);max-width:520px;margin:0 auto;">
    Open-source CLI for delivery governance. Every decision, requirement and roadmap
    linked — so your team (and your agents) always know why a change was made.
  </p>
  <div class="hero-actions">
    <a class="btn-brand" href="/trackfw/guide/getting-started">Get started →</a>
    <a class="btn-alt" href="https://github.com/kgsaran/trackfw">View on GitHub</a>
  </div>
</div>

<hr class="section-divider">

<!-- ============================================================
  PROBLEM
  ============================================================ -->
<div class="section">
  <div class="problem-block">
    <p>Most teams accumulate technical debt not because they lack tools,<br>
    but because they lack governance traceability.</p>
    <p>Decisions are made in Slack. Requirements live in someone's head.<br>
    Roadmaps drift from what was actually shipped.</p>
    <p><strong>trackfw closes the loop</strong> — the connective tissue between <em>why</em>, <em>what</em>, and <em>when</em>.</p>
  </div>
</div>

<hr class="section-divider">

<!-- ============================================================
  GOVERNANCE CHAIN (flow diagram — 2×3 grid)
  ============================================================ -->
<div class="section">
  <div class="section-title">The governance chain</div>

  <div class="chain-diagram" aria-label="Governance chain: ADR to PR in six steps">
    <div class="chain-row">
      <div class="chain-node">
        <code>trackfw adr new</code>
        <div>ADR — <em>why</em></div>
      </div>
      <span class="chain-arrow" aria-hidden="true">→</span>
      <div class="chain-node">
        <code>trackfw req new</code>
        <div>REQ — <em>what</em></div>
      </div>
      <span class="chain-arrow" aria-hidden="true">→</span>
      <div class="chain-node">
        <code>trackfw roadmap move … wip</code>
        <div>ROADMAP</div>
      </div>
    </div>
    <div class="chain-down" aria-hidden="true">↓</div>
    <div class="chain-row">
      <div class="chain-node">
        <div>Wave N</div>
        <div style="font-size:0.8rem;color:var(--vp-c-text-2)">ML-NA · ML-NB</div>
      </div>
      <span class="chain-arrow" aria-hidden="true">→</span>
      <div class="chain-node">
        <code>trackfw barrier</code>
        <div>5 checks before next wave</div>
      </div>
      <span class="chain-arrow" aria-hidden="true">→</span>
      <div class="chain-node">
        <code>trackfw ship</code>
        <div>PR / MR</div>
      </div>
    </div>
  </div>

  <p class="chain-learn">
    Guards (<code>git-branch</code>, <code>write-scope</code>, <code>credential</code>) run at every AI agent tool call.
    <a href="/trackfw/guide/getting-started">Learn how the chain works →</a>
  </p>
</div>

<hr class="section-divider">

<!-- ============================================================
  SCENARIOS
  ============================================================ -->
<div class="section">
  <div class="section-title">Fits how you work</div>

  <div class="scenarios-grid">
    <div class="scenario-card">
      <h3>Solo dev + AI agent</h3>
      <p>You use Claude Code or Codex. Your agent ships fast — trackfw makes it traceable. Guard hooks block raw git operations; <code>trackfw context --format=json</code> feeds the full governance state to the agent.</p>
      <img
        class="terminal-img"
        src="/img/guard-poster.png"
        alt="trackfw guard blocking a raw git push sent by an AI agent tool call — exits with code 2"
      >
      <details style="margin-top:0.75rem;font-size:0.85rem;">
        <summary>Play the full animation</summary>
        <img
          class="terminal-img"
          src="/img/guard.gif"
          alt="trackfw guard blocking a raw git push sent by an AI agent tool call — exits with code 2"
        >
      </details>
      <a class="scenario-link" href="/trackfw/guide/ai-agents">See the commands →</a>
    </div>
    <div class="scenario-card">
      <h3>Team with CI</h3>
      <p><code>trackfw validate</code> runs as a CI gate on every PR. No ADR? No merge. <code>trackfw barrier</code> checks wave completion before the next wave starts.</p>
      <a class="scenario-link" href="/trackfw/guide/commands">See the commands →</a>
    </div>
    <div class="scenario-card">
      <h3>Policy-restricted env</h3>
      <p>Can't <code>curl</code> executables to your machines? trackfw ships via <code>npm</code> and <code>pip</code> too — the same Go binary, delivered through the channel your policy already allows.</p>
      <a class="scenario-link" href="/trackfw/guide/getting-started#installation">Read the caveats →</a>
    </div>
  </div>
</div>

<hr class="section-divider">

<!-- ============================================================
  BOARD SCREENSHOT
  ============================================================ -->
<div class="section board-section">
  <div class="section-title">Your governance board, local and instant</div>
  <p style="color:var(--vp-c-text-2);margin-bottom:1rem;">
    <code>trackfw serve</code> opens a local Kanban board — no cloud, no account, no signup.
  </p>
  <img
    src="/img/board.png"
    alt="trackfw serve Kanban board showing roadmaps in backlog, analyzing, wip, blocked, done and abandoned columns"
    width="1600"
    height="560"
  >
</div>

<hr class="section-divider">

<!-- ============================================================
  DEMO VIDEO
  ============================================================ -->
<div class="section demo-section">
  <div class="section-title">See it in action</div>
  <p style="color:var(--vp-c-text-2);margin-bottom:1rem;">
    From <code>init</code> to a governed commit — in under two minutes.
  </p>
  <video
    controls
    width="1200"
    poster="/img/demo-poster.png"
    style="max-width:100%"
    aria-label="trackfw demo: ADR, requirement, roadmap move, validate, guard blocking a raw git push, status"
  >
    <source src="/img/demo.webm" type="video/webm">
    Your browser does not support the video element. See the
    <a href="https://raw.githubusercontent.com/kgsaran/trackfw/main/docs/demo.gif">animated GIF version</a>.
  </video>
  <details style="margin-top:1rem;font-size:0.85rem;">
    <summary>Transcript</summary>
    <ol>
      <li>Project set up with: <code>trackfw init --ai-tools claude --forge github</code></li>
      <li>Record a decision: <code>trackfw adr new "User Authentication Strategy"</code> — wizard answered with context, decision, and rejected alternatives.</li>
      <li>Linked requirement: <code>trackfw req new "User Authentication" --adr ADR-…-user-authentication-strategy.md</code></li>
      <li>Start the work: <code>trackfw roadmap move ROADMAP-…-user-authentication.md wip</code></li>
      <li>Validate: <code>trackfw validate</code> — 2 warnings (empty ADR and REQ sections), 0 violations</li>
      <li>AI agent blocked: <code>echo '{"tool_name":"Bash",…}' | trackfw guard git-branch</code> exits 2</li>
      <li>Status: <code>trackfw status</code></li>
    </ol>
  </details>
</div>

<hr class="section-divider">

<!-- ============================================================
  FEATURES
  ============================================================ -->
<div class="section">
  <div class="section-title">What you get</div>

  <div class="features-grid">
    <div class="feature-card">
      <div class="icon">🔗</div>
      <h3>Governance chain</h3>
      <p>ADR → REQ → ROADMAP → backlog/wip/done. Every line of code traces back to a decision.</p>
    </div>
    <div class="feature-card">
      <div class="icon">🤖</div>
      <h3>AI-agent native</h3>
      <p>Guard hooks block raw git operations in every major AI CLI. <code>trackfw context --format=json</code> feeds the full governance state to your agent at session start.</p>
    </div>
    <div class="feature-card">
      <div class="icon">🌐</div>
      <h3>Multi-stack, single binary</h3>
      <p>Go, Java, Node, Python. React/Vue/Angular. husky, lefthook, GitHub Actions, GitLab CI. One Go binary, delivered via npm, pip, Homebrew or curl.</p>
    </div>
    <div class="feature-card">
      <div class="icon">📦</div>
      <h3>No lock-in</h3>
      <p>Markdown files in your own repo. No server, no database, no mandatory account. <code>trackfw serve</code> gives you a local Kanban board — no cloud required.</p>
    </div>
  </div>
</div>

<hr class="section-divider">

<!-- ============================================================
  QUICK START
  ============================================================ -->
<div class="section">
  <div class="section-title">Quick start</div>

```bash
npm install -g trackfw            # or: pip install trackfw · brew install kgsaran/trackfw/trackfw

trackfw init --ai-tools claude --forge github
trackfw adr new "User Authentication Strategy"
trackfw req new "User Authentication" --adr ADR-$(date +%F)-user-authentication-strategy.md
trackfw roadmap move ROADMAP-$(date +%F)-user-authentication.md wip
trackfw validate                  # 0 violations, 2 warning(s) until you fill in the ADR and REQ sections
```

  <p style="font-size:0.9rem;color:var(--vp-c-text-2);margin-top:1rem;">
    In a terminal, <code>init</code>, <code>adr new</code> and <code>req new</code> open short wizards.
    This sequence is pinned by a test, so it keeps working as the product changes.
  </p>
  <img
    class="terminal-img"
    src="/img/quickstart-poster.png"
    alt="Terminal recording: trackfw init, adr new, req new, roadmap move to wip, validate showing 0 violations 2 warnings"
  >
  <details style="margin-top:0.75rem;font-size:0.85rem;">
    <summary>Play the full animation</summary>
    <img
      class="terminal-img"
      src="/img/quickstart.gif"
      alt="Terminal recording: trackfw init, adr new non-interactively, req new non-interactively, roadmap move to wip, validate showing 2 warnings 0 violations"
    >
  </details>
  <details style="margin-top:0.5rem;font-size:0.85rem;">
    <summary>Transcript</summary>
    <ol>
      <li><code>trackfw init --ai-tools claude --forge github</code> — project set up.</li>
      <li><code>trackfw adr new "User Authentication Strategy"</code> — ADR created.</li>
      <li><code>trackfw req new "User Authentication" --adr ADR-…-user-authentication-strategy.md</code> — linked requirement created.</li>
      <li><code>trackfw roadmap move ROADMAP-…-user-authentication.md wip</code> — roadmap moved to wip.</li>
      <li><code>trackfw validate</code> — 0 violations, 2 warnings (unfilled ADR and REQ sections).</li>
    </ol>
  </details>
  <a href="/trackfw/guide/getting-started" style="font-weight:600;color:var(--vp-c-brand-1);display:block;margin-top:0.75rem;">Read the full guide →</a>
</div>
