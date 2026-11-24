---
layout: doc
aside: false
title: "trackfw — Governança de entrega para agentes de IA e times"
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
/* Chain diagram — 2×3 grid no desktop, coluna única no mobile */
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
  <h1>Seu agente de IA entrega rápido.<br>trackfw faz ele entregar com rastreabilidade.</h1>
  <p style="font-size:1.1rem;color:var(--vp-c-text-2);max-width:520px;margin:0 auto;">
    CLI open-source para governança de entrega. Cada decisão, requisito e roadmap
    vinculados — para o seu time (e seus agentes) sempre saberem o porquê de cada mudança.
  </p>
  <div class="hero-actions">
    <a class="btn-brand" href="/trackfw/pt/guide/getting-started">Começar agora →</a>
    <a class="btn-alt" href="https://github.com/kgsaran/trackfw">Ver no GitHub</a>
  </div>
</div>

<hr class="section-divider">

<!-- ============================================================
  PROBLEM
  ============================================================ -->
<div class="section">
  <div class="problem-block">
    <p>A maioria dos times acumula dívida técnica não porque faltam ferramentas,<br>
    mas porque falta rastreabilidade de governança.</p>
    <p>Decisões ficam no Slack. Requisitos vivem na cabeça de alguém.<br>
    Roadmaps divergem do que foi realmente entregue.</p>
    <p><strong>trackfw fecha o ciclo</strong> — o tecido conectivo entre o <em>porquê</em>, o <em>quê</em> e o <em>quando</em>.</p>
  </div>
</div>

<hr class="section-divider">

<!-- ============================================================
  GOVERNANCE CHAIN (flow diagram — 2×3 grid)
  ============================================================ -->
<div class="section">
  <div class="section-title">A cadeia de governança</div>

  <div class="chain-diagram" aria-label="Cadeia de governança: do ADR ao PR em seis etapas">
    <div class="chain-row">
      <div class="chain-node">
        <code>trackfw adr new</code>
        <div>ADR — <em>porquê</em></div>
      </div>
      <span class="chain-arrow" aria-hidden="true">→</span>
      <div class="chain-node">
        <code>trackfw req new</code>
        <div>REQ — <em>o quê</em></div>
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
        <div>5 verificações antes da próxima wave</div>
      </div>
      <span class="chain-arrow" aria-hidden="true">→</span>
      <div class="chain-node">
        <code>trackfw ship</code>
        <div>PR / MR</div>
      </div>
    </div>
  </div>

  <p class="chain-learn">
    Guards (<code>git-branch</code>, <code>write-scope</code>, <code>credential</code>) rodam em cada chamada de ferramenta do agente.
    <a href="/trackfw/pt/guide/getting-started">Veja como a cadeia funciona →</a>
  </p>
</div>

<hr class="section-divider">

<!-- ============================================================
  SCENARIOS
  ============================================================ -->
<div class="section">
  <div class="section-title">Funciona do seu jeito</div>

  <div class="scenarios-grid">
    <div class="scenario-card">
      <h3>Dev solo + agente de IA</h3>
      <p>Você usa Claude Code ou Codex. Seu agente entrega rápido — trackfw faz isso com rastreabilidade. Guards bloqueiam operações git diretas; <code>trackfw context --format=json</code> alimenta o estado completo de governança para o agente.</p>
      <img
        class="terminal-img"
        src="/img/guard-poster.png"
        alt="trackfw guard bloqueando um git push direto na main enviado por ferramenta de agente — sai com código 2"
      >
      <details style="margin-top:0.75rem;font-size:0.85rem;">
        <summary>Reproduzir a animação completa</summary>
        <img
          class="terminal-img"
          src="/img/guard.gif"
          alt="trackfw guard bloqueando um git push direto na main enviado por ferramenta de agente — sai com código 2"
        >
      </details>
      <a class="scenario-link" href="/trackfw/pt/guide/ai-agents">Ver os comandos →</a>
    </div>
    <div class="scenario-card">
      <h3>Time com CI</h3>
      <p><code>trackfw validate</code> roda como gate de CI em cada PR. Sem ADR? Sem merge. <code>trackfw barrier</code> verifica a conclusão da wave antes da próxima começar.</p>
      <a class="scenario-link" href="/trackfw/pt/guide/commands">Ver os comandos →</a>
    </div>
    <div class="scenario-card">
      <h3>Ambiente com restrições de política</h3>
      <p>Não pode baixar executáveis? trackfw também distribui via <code>npm</code> e <code>pip</code> — o mesmo binário Go, entregue pelo canal que sua política já permite.</p>
      <a class="scenario-link" href="/trackfw/pt/guide/getting-started#installation">Leia as ressalvas →</a>
    </div>
  </div>
</div>

<hr class="section-divider">

<!-- ============================================================
  BOARD SCREENSHOT
  ============================================================ -->
<div class="section board-section">
  <div class="section-title">Seu board de governança, local e instantâneo</div>
  <p style="color:var(--vp-c-text-2);margin-bottom:1rem;">
    <code>trackfw serve</code> abre um board Kanban local — sem cloud, sem conta, sem cadastro.
  </p>
  <img
    src="/img/board.png"
    alt="Board Kanban do trackfw serve mostrando roadmaps em backlog, analyzing, wip, blocked, done e abandoned"
    width="1600"
    height="560"
  >
</div>

<hr class="section-divider">

<!-- ============================================================
  DEMO VIDEO
  ============================================================ -->
<div class="section demo-section">
  <div class="section-title">Veja em ação</div>
  <p style="color:var(--vp-c-text-2);margin-bottom:1rem;">
    De <code>init</code> a um commit governado — em menos de dois minutos.
  </p>
  <video
    controls
    width="1200"
    poster="/img/demo-poster.png"
    style="max-width:100%"
    aria-label="Demo do trackfw: ADR, requisito, roadmap move, validate, guard bloqueando git push direto, status"
  >
    <source src="/img/demo.webm" type="video/webm">
    Seu navegador não suporta o elemento de vídeo. Veja a
    <a href="https://raw.githubusercontent.com/kgsaran/trackfw/main/docs/demo.gif">versão em GIF animado</a>.
  </video>
  <details style="margin-top:1rem;font-size:0.85rem;">
    <summary>Transcrição</summary>
    <ol>
      <li>Projeto configurado com: <code>trackfw init --ai-tools claude --forge github</code></li>
      <li>Registrar decisão: <code>trackfw adr new "User Authentication Strategy"</code> — wizard respondido com contexto, decisão e alternativas rejeitadas.</li>
      <li>Requisito vinculado: <code>trackfw req new "User Authentication" --adr ADR-…-user-authentication-strategy.md</code></li>
      <li>Iniciar o trabalho: <code>trackfw roadmap move ROADMAP-…-user-authentication.md wip</code></li>
      <li>Validar: <code>trackfw validate</code> — 2 warnings (seções vazias do ADR e REQ), 0 violations</li>
      <li>Agente de IA bloqueado: <code>echo '{"tool_name":"Bash",…}' | trackfw guard git-branch</code> sai com código 2</li>
      <li>Status: <code>trackfw status</code></li>
    </ol>
  </details>
</div>

<hr class="section-divider">

<!-- ============================================================
  FEATURES
  ============================================================ -->
<div class="section">
  <div class="section-title">O que você ganha</div>

  <div class="features-grid">
    <div class="feature-card">
      <div class="icon" aria-hidden="true">🔗</div>
      <h3>Cadeia de governança</h3>
      <p>ADR → REQ → ROADMAP → backlog/wip/done. Cada linha de código rastreada até uma decisão.</p>
    </div>
    <div class="feature-card">
      <div class="icon" aria-hidden="true">🤖</div>
      <h3>Nativo para agentes de IA</h3>
      <p>Guards bloqueiam operações git em todos os principais CLIs de IA. <code>trackfw context --format=json</code> alimenta o estado completo de governança ao início de cada sessão.</p>
    </div>
    <div class="feature-card">
      <div class="icon" aria-hidden="true">🌐</div>
      <h3>Multi-stack, binário único</h3>
      <p>Go, Java, Node, Python. React/Vue/Angular. husky, lefthook, GitHub Actions, GitLab CI. Um binário Go, distribuído via npm, pip, Homebrew ou curl.</p>
    </div>
    <div class="feature-card">
      <div class="icon" aria-hidden="true">📦</div>
      <h3>Sem lock-in</h3>
      <p>Arquivos Markdown no seu próprio repositório. Sem servidor, sem banco de dados, sem conta obrigatória. <code>trackfw serve</code> oferece um board Kanban local — sem cloud.</p>
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
npm install -g trackfw            # ou: pip install trackfw · brew install kgsaran/trackfw/trackfw

trackfw init --ai-tools claude --forge github
trackfw adr new "User Authentication Strategy"
trackfw req new "User Authentication" --adr ADR-$(date +%F)-user-authentication-strategy.md
trackfw roadmap move ROADMAP-$(date +%F)-user-authentication.md wip
trackfw validate                  # 0 violations, 2 warning(s) until you fill in the ADR and REQ sections
```

  <p style="font-size:0.9rem;color:var(--vp-c-text-2);margin-top:1rem;">
    No terminal, <code>init</code>, <code>adr new</code> e <code>req new</code> abrem wizards curtos.
    Esta sequência é fixada por um teste, então continua funcionando à medida que o produto evolui.
  </p>
  <img
    class="terminal-img"
    src="/img/quickstart-poster.png"
    alt="Gravação de terminal: trackfw init, adr new, req new, roadmap move para wip, validate mostrando 0 violations 2 warnings"
  >
  <details style="margin-top:0.75rem;font-size:0.85rem;">
    <summary>Reproduzir a animação completa</summary>
    <img
      class="terminal-img"
      src="/img/quickstart.gif"
      alt="Gravação de terminal: trackfw init, adr new, req new, roadmap move para wip, validate mostrando 2 warnings 0 violations"
    >
  </details>
  <details style="margin-top:0.5rem;font-size:0.85rem;">
    <summary>Transcrição</summary>
    <ol>
      <li><code>trackfw init --ai-tools claude --forge github</code> — projeto configurado.</li>
      <li><code>trackfw adr new "User Authentication Strategy"</code> — ADR criado.</li>
      <li><code>trackfw req new "User Authentication" --adr ADR-…-user-authentication-strategy.md</code> — requisito vinculado criado.</li>
      <li><code>trackfw roadmap move ROADMAP-…-user-authentication.md wip</code> — roadmap movido para wip.</li>
      <li><code>trackfw validate</code> — 0 violations, 2 warnings (seções vazias do ADR e REQ).</li>
    </ol>
  </details>
  <a href="/trackfw/pt/guide/getting-started" style="font-weight:600;color:var(--vp-c-brand-1);display:block;margin-top:0.75rem;">Leia o guia completo →</a>
</div>
