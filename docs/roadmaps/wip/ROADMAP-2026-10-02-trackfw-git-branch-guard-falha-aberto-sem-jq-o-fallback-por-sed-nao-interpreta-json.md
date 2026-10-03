---
status: wip
date: 2026-10-02
req: "docs/req/REQ-2026-10-02-trackfw-git-branch-guard-falha-aberto-sem-jq-o-fallback-por-sed-nao-interpreta-json.md"
squad: "hades-tf, apolo-tf, hefesto-tf"
---

# Roadmap: trackfw-git-branch-guard falha aberto sem jq — o fallback por sed não interpreta JSON

> Created: 2026-10-02 | Status: wip

## Context
REQ: docs/req/REQ-2026-10-02-trackfw-git-branch-guard-falha-aberto-sem-jq-o-fallback-por-sed-nao-interpreta-json.md
ADR: docs/adr/ADR-2026-10-02-o-guard-de-branch-extrai-o-comando-do-payload-por-um-parser-json-de-verdade-e-falha-fechado-quando-nao-consegue.md
Issue: #507 (o PR fecha). Base: `main` em `3b2eff09`.

| sítio | papel |
|---|---|
| `internal/generators/scaffold.go` `gitBranchGuardScript` (~:1713; extração ~:1875-1879 do Go = linhas 163-176 do script) | **fonte** |
| `internal/validator/validator_git_branch_guard_reference.go` `gitBranchGuardScriptReference` | cópia (integridade) |
| `scripts/trackfw-git-branch-guard.sh` | cópia (hook vivo deste repositório) |
| `scripts/check-gates-falsify.sh`, cenário que sabota o guard (`corrupt_literal`) | literal que tem de casar exatamente 1 vez |
| `internal/generators/git_branch_guard_test.go` | tabela de testes, hoje só com `jq` |

Notas do vault obrigatórias para quem mexer: `guard-aprova-quando-nao-conseguiu-ler-o-comando-orcamento-total-do-read-2026-09-24`
(o dreno vive em 4 sítios byte-idênticos; o `corrupt_literal` já quebrou o gate 2 vezes) e
`rodar-um-unico-cenario-de-check-gates-falsify-e-provar-que-a-sabotagem-nao-e-vacua-2026-09-24`.

## Acceptance Criteria
- [x] AC1 — Wave 0 auditada
      ✅ `docs/seguranca/2026-10-02-wave0-extrator-json-do-guard.md`: APROVA COM AJUSTES. Formas de falha aberta confirmadas: `\n`, `\"`, `\t`, `\u000a`; e NUL, que passa até com `jq` (absorvido como AC5-bis/D2-bis).
- [ ] AC2 — multilinha e `\"` sem `jq` → rc=2; reprova em `3b2eff09`
- [ ] AC3 — `-m "a\nb"` literal: mesmo veredito com e sem `jq`
- [ ] AC4 — tabela inteira do guard com e sem `jq`
- [ ] AC5 — indecodificável → rc=2 nomeado; chave ausente → como hoje
- [ ] AC6 — 4 cópias iguais; cenário de falsificação segue provando a sabotagem
- [ ] AC7 — teste novo declara o que afirma
- [ ] AC8 — `make quality` EXIT=0 e CI verde

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Modelo de ameaça do extrator JSON em awk
**Status:** ✅ Concluído
**Squad:** hades-tf
**Files affected:** `docs/seguranca/2026-10-02-wave0-extrator-json-do-guard.md` (único arquivo escrito)
**Actions:**
1. **Completude:** confirme por efeito as duas formas de falha aberta (multilinha; `\"` truncando) com o script de `3b2eff09` e um `PATH` sem `jq`. Há outras? (por exemplo, `}` dentro da string antes da chave `command`, que o `[^}]*` da regex também corta).
2. **Ameaça ao extrator novo:** que payload faz um extrator `awk` escolher o valor errado ou decodificar errado: chave `command` dentro de outra string; chave duplicada; `tool_input` aninhado; `\\"`; contrabarra final; `\u0000`; `\u000a`/`\u000d` (newline por `\u`); `\ud83d` (surrogate); 200 KB. Para cada um, diga qual decisão o extrator tem de tomar para não falhar aberto.
3. **Diferenças entre `awk`:** BWK awk (macOS), gawk e mawk (Git Bash/Linux): `substr`, `index`, `split` com string vazia, `printf "%c"` com número. Meça no macOS o que puder; declare o resto como não medido.
4. **Custo:** tempo de extração com payload de 200 KB, `sed` contra um protótipo `awk` descartável (no scratch, não na árvore).
5. **Resíduo declarado.**
**Acceptance criteria:**
- [x] Seções com evidência (comando + saída)
- [x] Veredito explícito, com a lista de casos que a tabela de testes do ML-1A tem de conter
- [x] Nenhuma linha de implementação na árvore

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-02-wave0-extrator-json-do-guard.md
grep -q "Veredito" docs/seguranca/2026-10-02-wave0-extrator-json-do-guard.md
```

## Wave 1 — Implementação
> Dependencies: Wave 0 auditada

### ML-1A — Extrator JSON em awk nas 4 cópias e tabela com e sem jq
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Files affected:** os 5 sítios da tabela do Context
**Actions:** D1–D4, D2-bis e D2-ter do ADR. A tabela contém **os 21 casos C01–C21** da Wave 0 (§ tabela do parecer, com o rc esperado em cada caminho) **mais o C22**: `git push\u0000origin main` → rc=2 com e sem `jq`.
🔴 Pré-condição do teste sem `jq`: o macOS tem `/usr/bin/jq`, então o `PATH` curado **não pode** incluir `/usr/bin` inteiro; o teste afirma `command -v jq` vazio dentro do ambiente curado antes de rodar a tabela. Teste sem `jq` por `PATH` curado num `t.TempDir()` (padrão do `TestAttentionScripts_FallbackWithoutJQ`), rodando a tabela inteira duas vezes.
**Acceptance criteria:**
- [x] AC2–AC6 com testes por nome; prova de mordida contra o script de `3b2eff09`
- [x] Cenário de falsificação do guard rodado isolado (nota do vault) e provado não vácuo
- [x] Uma frase por teste novo
      ✅ Auditoria parcial (arquiteto): C01–C22 passam com e sem `jq`, e a prova de mordida contra `3b2eff09` mede falha aberta em C02–C05, C12 e C22. C09–C11 **não** falhavam no `sed` antigo (medido pelo agente; a lista do handoff era hipótese). As 4 cópias são iguais.
      ❌ AC4 não entregue: os 44 testes que já existiam no guard continuam rodando só com o `PATH` do sistema (com `jq`). Vai para o ML-1B.
**Gates da wave:**
```bash
go build ./...
go test ./internal/generators/ ./internal/validator/ -count=1
```

### ML-1B — Corretivo: a tabela que já existia roda também sem jq (AC4)
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Files affected:** `internal/generators/git_branch_guard_test.go`
**Actions:** `runGitBranchGuard` (~:140) executa o guard **duas vezes**, com o `PATH` do sistema e com o `PATH` curado sem `jq` do ML-1A (`makeCuratedPathWithoutJQ`, com `command -v jq` vazio afirmado), e reprova se o código de saída divergir. Devolve o resultado do caminho com `jq`. Assim todos os testes existentes passam a afirmar a equivalência dos dois extratores, incluindo o `TestGitBranchGuard_UnterminatedHeredocBeforeRealPush_StillBlocks` da #507. Testes que dependem de variáveis de ambiente específicas preservam essas variáveis nos dois modos.
**Acceptance criteria:**
- [x] Os 44 testes `TestGitBranchGuard*` passam, cada um rodando nos dois modos
- [x] Prova: com o script de `3b2eff09` no lugar do novo (overlay ou cópia), o `TestGitBranchGuard_UnterminatedHeredocBeforeRealPush_StillBlocks` reprova por divergência entre os modos
      ✅ 50/50 `TestGitBranchGuard*` passam (44 antigos agora nos dois modos). Contra o script de `3b2eff09` (overlay), o `UnterminatedHeredoc…` da #507 reprova por divergência: rc=2 com `jq`, rc=0 sem. Os 6 testes do dreno de stdin (outro arquivo) seguem só com `jq`: declarado.
**Gates da wave:**
```bash
go build ./...
go test ./internal/generators/ -run 'TestGitBranchGuard' -count=1
```

### ML-1C — Corretivo: o PATH curado sem jq tem de funcionar no Windows
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Origem:** relatório do ML-1B. `makeCuratedPathWithoutJQ` cria symlinks e, se `os.Symlink` falha (Windows sem Developer Mode, como nos runners), continua em silêncio com o diretório vazio. Agora os 44 testes antigos usam esse caminho, e o `windows-full-suites` roda `go test ./...`: seriam dezenas de nomes novos no ratchet.
**Files affected:** `internal/generators/git_branch_guard_test.go`
**Actions:** montar o ambiente sem `jq` **filtrando o PATH original**: tirar os diretórios que contêm um executável `jq` (`jq`, `jq.exe`). Para cada ferramenta de que o script precisa e que só existia num diretório removido (caso do `/usr/bin` do macOS), criar um shim num diretório próprio: symlink e, se falhar, cópia. Falha ao montar o ambiente → `t.Fatalf` com a causa, nunca diretório vazio em silêncio. Afirmar `command -v jq` vazio no ambiente final.
**Acceptance criteria:**
- [x] macOS: 50/50 `TestGitBranchGuard*` passam
- (movido) CI `windows-full-suites` sem nome novo no ratchet → é o AC8 da REQ, que só o CI do PR mede
      ✅ macOS: 50/50 passam (rodado pelo arquiteto, 22 s). `assertJQAbsentInPath` usa `t.Fatalf` (asserção real). Windows: o braço de cópia não foi medido localmente; o AC do Windows fecha com o CI do PR.
**Gates da wave:**
```bash
go build ./...
go test ./internal/generators/ -run 'TestGitBranchGuard' -count=1
```

### ML-1D — Corretivo: o nome da chave também é decodificado (achado N03 da revisão)
**Status:** ⬜ Pendente
**Squad:** apolo-tf
**Origem:** ML-2A. `{"tool_input":{"comm\u0061nd":"git push origin main"}}`: com `jq` rc=2, sem `jq` rc=0. O extrator `awk` guarda o nome da chave cru (`_lk[_d]=_raw`, ~linha 235 do script) e compara com `command` sem decodificar.
**Files affected:** os 4 sítios do Context; `internal/generators/git_branch_guard_test.go`
**Actions:** decodificar o nome da chave com a mesma função do valor antes de comparar; nome com escape inválido → nome cru (não casa). Acrescentar à tabela C01–C22 os casos **N01–N09** da revisão (`\u` incompleto, `\uzzzz`, chave escapada, valor número/null/array, `tool_input` string, 50 KB sem fechamento, 10 000 contrabarras) com o rc esperado nos dois caminhos.
**Acceptance criteria:**
- [ ] N03 rc=2 com e sem `jq`; reprova sem a correção (prova por overlay)
- [ ] N01–N09 na tabela, nos dois modos; 4 cópias iguais
**Gates da wave:**
```bash
go build ./...
go test ./internal/generators/ ./internal/validator/ -count=1
```

## Wave 2 — Revisão independente e gate completo (paralela)
> Dependencies: Wave 1 auditada

### ML-2A — Revisão de segurança
**Status:** ✅ Concluído
**Squad:** hades-tf
**Files affected:** `docs/seguranca/2026-10-02-wave2-revisao-extrator-json-do-guard.md`
**Actions:** reimplementar a partir da leitura os payloads do ML-0A contra o script novo, com e sem `jq`.
**Acceptance criteria:**
- [x] Veredito explícito
      ✅ APROVA COM AJUSTES: C01–C22 sem divergência jq×awk; achado novo **N03** (chave com escape unicode no nome, `"comm\\u0061nd"`): o `jq` decodifica e bloqueia, o `awk` não e deixa passar → ML-1D. Resíduo R1 do parecer está desatualizado (NUL já nega nos dois caminhos; nota do arquiteto no parecer).

### ML-2B — Revisão de qualidade e `make quality`
**Status:** ⬜ Pendente
**Squad:** hefesto-tf
**Files affected:** `docs/qualidade/2026-10-02-revisao-extrator-json-do-guard.md`
**Actions:** revisão; `make quality` com a máquina ociosa, citando do log a linha da suíte de falsificação.
**Acceptance criteria:**
- [ ] `make quality` EXIT=0
- [ ] Veredito explícito

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-02-wave2-revisao-extrator-json-do-guard.md
test -s docs/qualidade/2026-10-02-revisao-extrator-json-do-guard.md
```
