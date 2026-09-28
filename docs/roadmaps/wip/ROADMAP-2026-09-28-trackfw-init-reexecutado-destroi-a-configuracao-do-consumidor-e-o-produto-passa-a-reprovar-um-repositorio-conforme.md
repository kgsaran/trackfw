---
status: wip
date: 2026-09-28
req: "docs/req/REQ-2026-09-28-trackfw-init-reexecutado-destroi-a-configuracao-do-consumidor-e-o-produto-passa-a-reprovar-um-repositorio-conforme.md"
squad: ""
---

# Roadmap: `trackfw init` reexecutado destrói a configuração do consumidor

> Created: 2026-09-28 | Status: wip

## Context
<!-- Derived from REQ -->
REQ: docs/req/REQ-2026-09-28-trackfw-init-reexecutado-destroi-a-configuracao-do-consumidor-e-o-produto-passa-a-reprovar-um-repositorio-conforme.md
ADR: docs/adr/ADR-2026-09-28-trackfw-init-reexecutado-preserva-a-configuracao-autorada-pelo-consumidor.md
Origem: **#445** + ocorrência real neste repositório (2026-09-27), `validate` 170 warnings → 156 violations.

## Acceptance Criteria
<!-- Consolidados; detalhe por ML nas waves abaixo. -->
- [ ] Os 22 sítios enumerados e classificados em (a)/(b)/(c), com a razão escrita por sítio
- [ ] `init` reexecutado preserva `governance_mode`, `lenient_until` e `agent_models` — e a saída de
      `trackfw validate` é **byte-idêntica** antes e depois
- [ ] Comentários preservados, incluindo a justificativa de cota do bloco `agent_models`
- [ ] Zero diff nas linhas pré-existentes; chave nova de versão nova **É** acrescentada (contra-braço)
- [ ] Gate falsificável nas duas direções: reprova sítio (a) novo que trunque; **não** reprova (b) legítimo
- [ ] `make quality` e CI verdes

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat model: medir o que é defeito e o que é comportamento desejado
> Dependências: nenhuma. 🔴 **Bloqueia toda a implementação.**

**Gates da wave:**

```bash
n=$(grep -cE '^[[:space:]]*(if err := )?os\.(WriteFile|OpenFile)' internal/generators/scaffold.go); test "$n" = "22" && echo "Gate W0: $n sitios reais de escrita (regua com ancora de inicio de linha)" || { echo "GATE FALHOU: esperava 22 sitios, contou $n — a populacao mudou, reclassifique antes de implementar" >&2; exit 1; }
```

### ML-0A — enumerar e classificar os 22 sítios de escrita
**Owner:** `hades-tf`
**Status:** ⬜ Pendente
**Arquivos:** `internal/generators/scaffold.go` (leitura), `docs/seguranca/2026-09-28-wave0-init-destroi-config.md` (escrita)

**Tarefa:** para cada um dos 22 sítios reais de `os.WriteFile`/`os.OpenFile`, classificar em:
- **(a)** config/declaração **autorada pelo consumidor** → truncar é **defeito**
- **(b)** artefato **gerado pelo produto** → truncar é **correto**, é como a correção chega
- **(c)** **híbrido** (bloco gerado dentro de arquivo do usuário) → verificar se o ramo de merge está completo

🔴 **A régua é a natureza do conteúdo destruído, NÃO a forma da chamada.** Um sítio (b) "corrigido"
para preservar congelaria scripts defeituosos na máquina de quem instalou — foi por (b) que o fix de
CRLF do #353 chegou aos consumidores.

**Critérios de aceite:**
- [ ] Os 22 sítios enumerados com linha exata e classe, **nenhum sem razão escrita**
- [ ] A população **(a)** nomeada; se for só `trackfw.yaml`, dizer isso explicitamente
- [ ] Os sítios **(c)** verificados: o ramo de merge existente está **completo** ou tem lacuna medida?
- [ ] 🔴 **Refutação ou confirmação da premissa da ADR** de que merge textual basta — se algum sítio
      (a) tiver YAML aninhado/listas que o merge textual não cobre, **dizer agora**, não na Wave 1

## Wave 1 — o merge textual, e o gate que o sustenta
> Dependências: **Wave 0 auditada.** MLs 1A e 1B tocam arquivos disjuntos → paralelos.

### ML-1A — `writeTrackfwConfig` mescla em vez de truncar
**Owner:** `apolo-tf`
**Status:** ⬜ Pendente
**Arquivos:** `internal/generators/scaffold.go`, teste novo em `internal/generators/`
**⚠️ NÃO toque em `scripts/`** — é do ML-1B, que roda em paralelo.

**Critérios de aceite:**
- [ ] `trackfw.yaml` ausente → escreve o template inteiro (comportamento atual preservado)
- [ ] `trackfw.yaml` presente → preserva **todo** valor existente e acrescenta **só** chaves ausentes
- [ ] 🔴 Comentários preservados, inclusive comentário de linha ao lado de valor
- [ ] 🔴 **Zero diff** nas linhas pré-existentes quando não há chave nova a acrescentar
- [ ] **Contra-braço:** chave nova de versão nova **É** acrescentada — sem ele, "preservar" degenera
      em "não escrever nada"
- [ ] 🔴 **O teste que mede o efeito:** fixture com `governance_mode: lenient` + `lenient_until` +
      `agent_models` comentado; `validate` **byte-idêntico** antes e depois do `init`
- [ ] Reconciliação: cada teste novo declara, em uma frase, qual conclusão do ML afirma

### ML-1B — gate que impede a reintrodução
**Owner:** `apolo-tf`
**Status:** ⬜ Pendente
**Arquivos:** `scripts/check-init-preserves-user-config.sh` (novo), `Makefile` se necessário
**⚠️ NÃO toque em `internal/generators/scaffold.go`** — é do ML-1A, que roda em paralelo.

**Critérios de aceite:**
- [ ] 🔴 **Falsificável nas duas direções:** reprova quando um sítio **(a)** novo nasce truncando;
      **não** reprova um sítio **(b)** legítimo — as duas execuções coladas no relatório
- [ ] Guarda de anti-vacuidade: o gate declara quantos sítios examinou e **reprova se examinar zero**
- [ ] A lista de sítios (b) isentos é **explícita e comentada**, não uma exclusão silenciosa

## Wave 2 — auditoria independente
> Dependências: Wave 1 completa e auditada.

### ML-2A — revisão independente por reimplementação
**Owner:** `hades-tf`
**Status:** ⬜ Pendente
**Tarefa:** reimplementar a verificação **a partir da leitura da ADR e da REQ**, não conferindo o
diff. A Regra Dura de Reconciliação pega contradição interna; **não** pega premissa errada
compartilhada entre teste e implementação. Esta wave existe para isso.

**Critérios de aceite:**
- [ ] O cenário do dano real reproduzido do zero: `governance_mode` + `lenient_until` + `agent_models`
      com comentário → `init` → `validate` byte-idêntico
- [ ] Veredito explícito sobre se algum sítio **(a)** ficou sem correção — a ADR de ponto único
      **não** está satisfeita enquanto sobrar sítio
