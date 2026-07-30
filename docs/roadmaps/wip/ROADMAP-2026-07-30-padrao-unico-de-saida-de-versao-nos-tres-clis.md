---
status: wip
date: 2026-07-30
req: "REQ-2026-07-30-padrao-unico-de-saida-de-versao-nos-tres-clis"
squad: ""
---

# Roadmap: padrao unico de saida de versao nos tres CLIs

> Created: 2026-07-30 | Status: wip

## Contexto

REQ: `docs/req/REQ-2026-07-30-padrao-unico-de-saida-de-versao-nos-tres-clis.md`

Duas divergências medidas na `v5.0.0`:

| Superfície | Go | Node.js | Python |
|---|---|---|---|
| `version` | `trackfw v5.0.0` | `trackfw 5.0.0` | `trackfw 5.0.0` |
| `--version` | `trackfw v5.0.0` | **`5.0.0`** | `trackfw 5.0.0` |

E o gate de paridade **assina** a do Node: `check-cli-parity.sh:108` usa regex própria para ele.

**Formato canônico decidido:** `trackfw <semver>`, **sem** prefixo `v`, idêntico nas duas superfícies e
nos três runtimes. A tag Git permanece `v<x.y.z>`.

## Critérios de Aceite

- [ ] `version` e `--version` imprimem a **mesma linha**, `trackfw <semver>` sem `v`, nos três runtimes.
- [ ] `internal/version/version.go` armazena a versão sem `v`.
- [ ] `check-cli-parity.sh` usa a mesma asserção para os três, exigindo o formato exato.
- [ ] Cenário byte-a-byte das duas superfícies em `make quality`, com prova de falsificação.
- [ ] `make quality` exit 0 e `validate --json` 0 violações.

## Mapa de dependências

```
Wave 1 — ML-1A (contrato, orquestrador)
   ↓ barrier
Wave 2 — ML-2A (Go) ‖ ML-2B (Node.js) ‖ ML-2C (Python)   ← spawn simultâneo, arquivos disjuntos
   ↓ barrier
Wave 3 — ML-3A (gate unificado + falsificação)
```

### Lição acumulada, aplicada ao ML-1A

O ML-1A falhou por **quatro** roadmaps seguidos pelo mesmo padrão: pinar a forma e deixar o
comportamento à interpretação — nomes sem valores, regexes sem o momento da validação, cardinalidades
sem ordem, ordem "de varredura" que não é ordem. Aqui o contrato pina o **texto literal exato** das duas
superfícies e a **asserção literal** que o gate deve usar, não a descrição do formato.

**Risco específico desta feature:** a asserção atual (`^trackfw .+`) é frouxa o bastante para aceitar as
duas formas. Se o ML-3A reaproveitar essa regex, o gate nasce vacuoso e o `v` volta sem ninguém ver. O
gate tem de comparar **bytes entre runtimes**, não casar um padrão permissivo.

---

## Wave 1 — Congelar o contrato (1 ML)
> Dependências: nenhuma

### ML-1A — Pinar o formato e a asserção do gate
**Status:** ✅ Concluído (contrato autorado pelo orquestrador)
**Agente:** orquestrador (`trackfw_architect`) — autoria exclusiva
**Arquivos afetados:** `docs/cli-parity.md` — linha `version` / `--version` da tabela de comandos e
seção própria

**Deve pinar:**
1. **Texto literal** das duas superfícies: `trackfw <semver>`, sem `v`, sem sufixo, uma linha, stdout.
2. **`version` e `--version` produzem saída idêntica** — mesma linha, byte a byte.
3. **Fonte da string** por runtime: Go de `internal/version/version.go` (sem `v`); Node.js de
   `npm/package.json`; Python de `importlib.metadata` com fallback literal.
4. **Asserção do gate**, literal: mesma para os três, exigindo `^trackfw [0-9]+\.[0-9]+\.[0-9]+$`, e
   **comparação byte-a-byte entre runtimes** — a regex sozinha não basta.
5. Registro de que a tag Git permanece `v<x.y.z>` e por quê.
6. Registro de que `-v` está **fora de escopo**, com o motivo.

**Seção escrita:** `## Version output` em `docs/cli-parity.md`, mais a linha `version` / `--version` da
tabela de comandos.

**Critérios de aceite:**
- [x] Texto literal das duas superfícies pinado — tabela com nome do programa, formato SemVer sem `v`,
      uma linha, stdout.
- [x] Equivalência `version` ≡ `--version` pinada como **byte-idêntica**, dentro e entre runtimes.
- [x] Asserção do gate pinada literalmente (`^trackfw [0-9]+\.[0-9]+\.[0-9]+$`) **mais** a exigência de
      comparação byte-a-byte — com o registro de por que a asserção anterior era vacuosa: `^trackfw .+`
      aceitava as duas formas, e o Node.js tinha regex própria que assinava a divergência.
- [x] Escopo negativo registrado: `-v` fora de escopo com o motivo, tag Git permanece `v<x.y.z>`,
      manifestos inalterados.
- [x] Fonte da string por runtime documentada, incluindo o fato de que no Go as duas superfícies
      consomem a mesma constante — logo uma única mudança corrige ambas.

---

## Wave 2 — Implementar nos três runtimes (3 MLs em paralelo)
> Dependências: ML-1A completo. Arquivos disjuntos — **spawn simultâneo**.

### ML-2A — Go
**Status:** ⬜ Pendente
**Agente:** Apolo
**Arquivos afetados:** `internal/version/version.go`, testes correspondentes

**Ação:** remover o `v` da constante. Isso corrige `version` e `--version` de uma vez, porque ambas
consomem `version.Version` (`internal/commands/version.go:15` e `internal/commands/root.go:22`).

**Critérios de aceite:**
- [ ] `version` e `--version` imprimem `trackfw <semver>` sem `v`, idênticos entre si.
- [ ] `go build ./...`, `go test ./...`, `go vet ./...` passam.

### ML-2B — Node.js
**Status:** ⬜ Pendente
**Agente:** Apolo
**Arquivos afetados:** `npm/src/commands/version.js` e/ou a configuração do commander, testes

**Ação:** fazer o `--version` imprimir `trackfw <semver>` em vez do número puro. O default do
`.version()` do commander imprime só o número — é preciso sobrescrever.

**Critérios de aceite:**
- [ ] `--version` passa a imprimir `trackfw <semver>`, idêntico ao `version`.
- [ ] `cd npm && npm test` passa.

### ML-2C — Python
**Status:** ⬜ Pendente
**Agente:** Apolo
**Arquivos afetados:** testes em `pypi/tests/`

**Ação:** o Python **já está no formato canônico** nas duas superfícies. Este ML é apenas cobertura:
adicionar teste que trave o formato exato, para que uma mudança futura no argparse não regrida em
silêncio.

**Critérios de aceite:**
- [ ] Teste travando o formato exato de `version` e `--version`.
- [ ] Nenhuma mudança de comportamento.
- [ ] Suíte Python passa.

---

## Wave 3 — Gate unificado e falsificação (1 ML)
> Dependências: **barrier** — ML-2A, ML-2B e ML-2C concluídos.

### ML-3A — Unificar a asserção e provar não-vacuidade
**Status:** ⬜ Pendente
**Agente:** Artemis

**Ações:**
1. **Remover a regex específica do Node.js** de `check-cli-parity.sh:108`. Os três passam a usar a mesma
   asserção literal pinada no contrato.
2. Cenário comparando **bytes** das duas superfícies entre os três runtimes.
3. Cenário de falsificação: seam que reintroduz o `v` num runtime e prova que o gate reprova. Corromper a
   **implementação**, nunca a asserção, com guarda de padrão contra `sed` obsoleto.

**Critérios de aceite:**
- [ ] Regex específica do Node.js removida; asserção única para os três.
- [ ] Comparação byte-a-byte das duas superfícies.
- [ ] Seam verificado por execução: com o `v` reintroduzido, o gate **falha**.
- [ ] `make quality` exit 0, `validate --json` 0 violações, `git status` limpo.
