---
name: agente-nao-roda-init-na-arvore
description: Handoff deve proibir rodar `trackfw init`/`discover --init` na árvore do projeto — o init reescreve o trackfw.yaml e muda o veredito do validate
metadata:
  type: feedback
---

🔴 **Todo handoff que peça para "provar que o artefato gerado funciona" tem de dizer, explicitamente:
gere em `mktemp -d`, NUNCA na árvore do projeto.**

**Why:** em 2026-09-27 um executor rodou `trackfw init` para provar que o `trackfw-attention-signal.sh`
gerado roda sem a `lib-crlf-normalize.sh`. O `init` **reescreveu o `trackfw.yaml` do repositório** e
apagou:

```
governance_mode: lenient  ·  lenient_until: "2027-12-31"
ci: github-actions → ci: none  ·  forge: github (sumiu)
todo o bloco agent_models (ADR-2026-08-21, com a justificativa de cota)
```

**O dano não foi só perder config.** Sem o `lenient`, os **170 warnings viraram 156 violations** e o
`validate` passou a sair `rc=1`. O executor então mediu esse número e reportou *"156 violations —
todas pré-existentes"*. **Não eram**: ele mediu o efeito da própria ação três comandos antes. E o
`make quality` que ele deixou rodando também rodou sobre a árvore corrompida — resultado inválido.

**How to apply:**

- No handoff: *"para provar que o gerado funciona, gere num `mktemp -d`. 🔴 Não rode `trackfw init`
  nem `discover --init` na árvore do projeto — eles reescrevem o `trackfw.yaml` e mudam o veredito
  do `validate`."*
- Na auditoria: **`git status --porcelain` antes de aceitar qualquer número** do relatório. Arquivo
  fora do escopo declarado do ML é sinal de efeito colateral, e números medidos depois dele são
  suspeitos. `trackfw.yaml`, `.gitignore` e `CLAUDE.md` são os que mais aparecem.
- Restaurar é `git show HEAD:<path> > <path>` — `git checkout --` está bloqueado pelo guard.

Isto é o **#445** (`trackfw init` re-executado destrói `roadmap_dir` customizado) acontecendo dentro
de casa. A issue tinha reprodução sintética; este caso deu dano real com a lista do que se perde.

Relacionado: [[o-instrumento-mente]] — medir depois de alterar o ambiente e chamar o resultado de
"pré-existente" é a mesma família.

**Reincidência (2026-10-09, REQ-2026-09-09 ML-6C):** mesmo com o handoff proibindo `init`/`req new`/`roadmap new`
na árvore, o red-team criou `docs/roadmaps/backlog/ROADMAP-2026-10-09-agent-feature.md` nela, e o meu `git add -A`
o commitou. A proibição no handoff não basta: **antes de `git add -A`, leia o `git status --short` e confira cada
arquivo novo contra a lista que o agente reportou** — arquivo `??` que ninguém declarou é sinal de vazamento.
