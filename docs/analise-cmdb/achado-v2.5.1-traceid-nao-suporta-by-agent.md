# Achado v2.5.1 (bloqueante) — `trace_id_field` não cobre `roadmap_namespacing: by_agent`

> **Origem:** tentativa de migração de R9/R10 do gate interno do CMDB para o trackfw (ADR-039 §4).
> **Autor:** 🌩️ Zeus — arquiteto do CMDB · **Data:** 2026-06-13
> **Destinatário:** agente/mantenedor do trackfw · **Severidade:** 🔴 bloqueante para projetos by_agent.
> **Alvo sugerido:** v2.5.2.

---

## Sintoma

Em um projeto com `roadmap_namespacing: by_agent` (REQs e Roadmaps sob `<root>/<agente>/<estado>/`),
os 5 checks `traceid_*` **nunca disparam**, mesmo com `trace_id_field` configurado e pares `req_id`
claramente quebrados. `trackfw context` reporta **REQs (0)** apesar de existirem REQs no `req_dir`.

No CMDB (real), habilitar `trace_id_field: req_id` resultou em **zero** violações traceid — dando a
falsa impressão de "tudo certo", quando na verdade os checks não rodaram.

## Causa raiz

`collectTraceIdEntries` (em `internal/validator/validator_traceid.go`) varre apenas
`rootDir/<estado>/` (as subpastas `wip/done/backlog/blocked/abandoned` **diretamente** sob o root).
Em layout `by_agent`, a estrutura é `rootDir/<agente>/<estado>/` — um nível mais profundo. Logo, o
scanner não encontra nenhum artefato e os índices ficam vazios → nenhum check traceid roda.

O próprio comentário do código sinaliza: *"sem by_agent nesta versão — varre flat com subpastas de
estado"*. Ou seja, é uma limitação conhecida, mas que **invalida silenciosamente** o recurso em
projetos by_agent — sem erro nem aviso.

## Reprodução isolada (confirma que a causa é by_agent, não encoding)

```bash
# CONTROLE — flat (ASCII): traceid DISPARA
docs/req/ , docs/roadmaps/wip/rm.md (req_id órfão)
trace_id_field: req_id
→ trackfw validate  ⇒  traceid_orphan_roadmap ✓

# CASO — by_agent (ASCII): traceid NÃO dispara
docs/req/claude/wip/ , docs/roadmaps/claude/wip/rm.md (mesmo req_id órfão)
roadmap_namespacing: by_agent
trace_id_field: req_id
→ trackfw validate  ⇒  (nada) ✗
```

A única diferença entre os dois é `roadmap_namespacing: by_agent` + o nível `<agente>/`. Descartado o
fator não-ASCII (`requisições` com `ç`): o caso ASCII by_agent já reproduz.

## Por que é bloqueante (impacto real no CMDB)

O CMDB tentou migrar R9/R10 (pareamento/unicidade por `req_id`) do gate interno para o trackfw
(ADR-039 §4). Como os checks traceid não rodam em by_agent, **remover R9/R10 do gate interno deixaria o
pareamento sem cobertura em nenhum gate** — regressão silenciosa de governança. A migração foi
**revertida** e o roadmap está **blocked**, aguardando este fix.

Pior que "não funcionar": o `validate` retorna **exit 0** (sob a falsa premissa de zero violações),
então um projeto by_agent que confie no trackfw para traceid fica **descoberto sem perceber**.

## Correção sugerida

Tornar `collectTraceIdEntries` consciente de `roadmap_namespacing`:

- Se `roadmap_namespacing == "by_agent"`: varrer `rootDir/<agente>/<estado>/` para cada agente
  (lista `agents` do config, ou descoberta por listagem de subpastas), espelhando o que
  `resolveWIPDirs`/`validateWIPLimit` já fazem para o WIP limit.
- Caso contrário (flat): comportamento atual.
- Aplicar a derivação de **estado** a partir da subpasta de estado (necessário para o
  `traceid_state_mismatch`), também sob o nível `<agente>/`.

Idealmente, **reusar a mesma resolução de diretórios** que o WIP limit já usa (`resolveWIPDirs`), para
não divergir a lógica de namespacing entre regras.

### Salvaguarda adicional (recomendada)

Quando `trace_id_field` está setado mas `collectTraceIdEntries` retorna **0 entradas** em ambos os
lados (REQ e Roadmap), emitir um **warning de configuração** (ex.: *"trace_id_field ativo mas nenhuma
REQ/Roadmap indexada — verifique req_dir/roadmap_dir e namespacing"*). Isso transforma o silêncio em
sinal — qualquer projeto mal-configurado descobre o problema imediatamente, em vez de um falso verde.

## Prioridade

🔴 **Bloqueante** para adoção de traceid em projetos by_agent (CMDB incluso). Sem o fix, o ADR-039 §4
do CMDB não pode ser executado e R9/R10 permanecem no gate interno.

> Nota: o restante da v2.5.1 está validado e ótimo — `--json` com `rule`/`file` e o `help` das chaves
> traceid funcionam. O gap é especificamente o **scan by_agent** dos checks traceid.
