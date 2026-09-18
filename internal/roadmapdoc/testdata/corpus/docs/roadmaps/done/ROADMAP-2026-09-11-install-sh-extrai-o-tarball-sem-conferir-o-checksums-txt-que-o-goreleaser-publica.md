---
status: done
date: 2026-09-11
req: "docs/req/REQ-2026-09-11-install-sh-extrai-o-tarball-sem-conferir-o-checksums-txt-que-o-goreleaser-publica.md"
squad: ""
---

# Roadmap: install.sh extrai o tarball sem conferir o checksums.txt que o goreleaser publica

> Created: 2026-09-11 | Status: done

## Context
<!-- Derived from REQ: REQ-2026-09-11-install-sh-extrai-o-tarball-sem-conferir-o-checksums-txt-que-o-goreleaser-publica.md -->
REQ: docs/req/REQ-2026-09-11-install-sh-extrai-o-tarball-sem-conferir-o-checksums-txt-que-o-goreleaser-publica.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ]
- [ ]

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** ✅ Concluído
**Executado em:** 2026-09-12, no fechamento do roadmap — **fora de ordem**. A Wave 1 foi entregue e
mergeada (#331) antes deste ML rodar. Registrado como desvio, não como sequência normal.

**1. Completude da enumeração — a lista fecha em UM sítio.**
Medido por busca no repositório, não por leitura da REQ:
```
grep -rnE "tar -x|tar\.gz|extractall|tarfile|releases/download" --include=*.{sh,go,js,py,yml}
  → scripts/install.sh:214   tar -xzf   ← único extrator
  → scripts/install.sh:99    releases/download/${VERSION}/${FILENAME}
  → scripts/check-install-*.sh          ← gates, não produto
```
Verificado que **não há segundo caminho de download**: `npm/package.json` não tem `postinstall`
(scripts = test, smoke); `pypi` não baixa binário; `update harness` escreve arquivo local e não
faz rede. O `third-party fetch` baixa, mas é outra superfície, governada por quarentena +
provenance — causa diferente, fora deste roadmap.

**2. Quem esvazia esta Wave sem quebrar regra escrita.**
O gate confere que `install.sh` **contém** a conferência. Um ataque de baixo custo é manter a
conferência presente e torná-la inócua — `set +e` antes dela, ou redirecionar o `exit 1`. Por isso
o gate não lê o texto: ele **executa** o instalador com um tarball adulterado de um byte e exige
que a instalação **falhe** (AC3), e com o tarball íntegro exige que **instale** (AC4). Um
verificador que só recusa reprova no contra-braço.

**3. Falsificação nas duas direções** — implementada, 9 cenários em
`scripts/check-install-checksum.sh`:

| direção | cenário | esperado |
|---|---|---|
| regressão | um byte trocado no tarball | instalação aborta |
| regressão | checksum ausente / duplicado / divergente | aborta, mensagem distinta |
| regressão | `checksums.txt` inexistente | aborta (não segue sem conferir) |
| oposta | tarball íntegro | instala normalmente |
| oposta | ambiente sem `sha256sum` (macOS) | usa `shasum -a 256` e instala |
| oposta | ausência das duas utilidades | **aborta**, nomeando a utilidade que falta |

**4. 🔴 Residual declarado — o `checksums.txt` vem da mesma origem que o tarball.**
Ele é baixado de `releases/download/<tag>/checksums.txt`, o mesmo host e a mesma tag do artefato
que ele autentica. Isso fecha **adulteração em trânsito de um** dos dois e **corrupção parcial**,
que era o defeito (H-02). **Não fecha** comprometimento do release no GitHub nem da conta que
publica: quem forja o tarball forja o `checksums.txt` junto. Fechar isso exige **assinatura**
(cosign/minisign) com a chave pública fora do canal de distribuição — não implementado, e não
prometido em lugar nenhum.

**Critérios de aceite:**
- [x] As quatro seções acima respondidas com evidência, não asserção de uma linha
- [x] Nenhuma linha de implementação escrita neste ML

**Gates da wave:**
```bash
scripts/check-install-checksum.sh   # 9 cenários; substitui o placeholder que falhava fechado
```

## Wave 1 — Implementation (derived from REQ criteria)
> Dependencies: none

### ML-1A — **AC1** — o instalador baixa o checksums.txt **da mesma tag**, valida o nome esperado e confere
**Status:** ✅ Concluído
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC1** — o instalador baixa o checksums.txt **da mesma tag**, valida o nome esperado e confere
- [ ] build passes
- [ ] tests green

### ML-1B — **AC2** — 🔴 **falha fechado** quando o checksum está **ausente**, **duplicado** ou **divergente**.
**Status:** ✅ Concluído
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC2** — 🔴 **falha fechado** quando o checksum está **ausente**, **duplicado** ou **divergente**.
- [ ] build passes
- [ ] tests green

### ML-1C — **AC3** — 🔴 **Falsificação:** stub de download que troca **um byte** do tarball ⇒ **nenhum
**Status:** ✅ Concluído
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC3** — 🔴 **Falsificação:** stub de download que troca **um byte** do tarball ⇒ **nenhum
- [ ] build passes
- [ ] tests green

### ML-1D — **AC4** — 🔴 **Contra-braço:** tarball íntegro instala normalmente. Verificador que só recusa é
**Status:** ✅ Concluído
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC4** — 🔴 **Contra-braço:** tarball íntegro instala normalmente. Verificador que só recusa é
- [ ] build passes
- [ ] tests green

### ML-1E — **AC5** — funciona onde não há sha256sum (macOS usa shasum -a 256). 🔴 **Ausência da
**Status:** ✅ Concluído
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC5** — funciona onde não há sha256sum (macOS usa shasum -a 256). 🔴 **Ausência da
- [ ] build passes
- [ ] tests green

### ML-1F — **AC6** — gate que reprova se o install.sh voltar a extrair sem conferir.
**Status:** ✅ Concluído
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC6** — gate que reprova se o install.sh voltar a extrair sem conferir.
- [ ] build passes
- [ ] tests green

---

## ✅ Entregue — 2026-09-11 · auditado pelo arquiteto

**Ordem verificada por leitura, não por relatório** (`scripts/install.sh`):

```
166  Erro: nem sha256sum nem shasum encontrados        AC5 — falha fechada
187  Erro: checksum ausente                            AC2 — estado 1
191  Erro: checksum duplicado                          AC2 — estado 2
204  Erro: checksum divergente                         AC2 — estado 3
211  Checksum OK: <hash>
214  tar -xzf                                          ← extração DEPOIS da conferência
```

`scripts/check-install-checksum.sh` → **9 cenários OK**, incluindo o braço de falsificação:
removendo a verificação, um binário **MALICIOUS é instalado e executa** — 🔴 prova de que o gate é
*load-bearing*, e não decorativo.

### 🔴 Quatro vacuidades que o próprio ML pegou no gate que ele mesmo escreveu

Vale registrar, porque é a disciplina que faltou em vários MLs de hoje:

1. **O `sed` de remoção apagava até o EOF** se o padrão final não casasse. O script mutilado saía 0 sem
   binário, e a asserção `RC=0 OU binário presente` reportava **OK por vacuidade**. Corrigido com
   guarda de sanidade: o mutilado **não** pode conter `Checksum OK` **e precisa** conter `tar -xzf`.
2. **O `grep` de "ferramenta de hash ausente" era largo demais** (`checksum\|hash\|verifica`) e casava
   com a mensagem de *falha de download* — uma falha de rede satisfaria vacuamente o cenário. Trocado
   pelo literal exato.
3. **O ramo `wget` nunca era exercitado** — todo cenário achava `curl` primeiro no PATH. Cenário C7 com
   PATH sem `curl`.
4. **A cláusula "mesma tag" do AC1 não era verificada.** Os stubs passaram a registrar cada URL pedida,
   e o cenário asserta `/v7.3.0/` na URL do `checksums.txt`.

### Nota de escopo

O `make parity-rest` reprova nesta worktree por **`direction-b2/node`**, e **não é desta frente**: a
branch nasceu antes do ML-1E-b. Rebase sobre a frente A resolve. Os arquivos desta frente
(`install.sh`, `check-install-checksum.sh`, `Makefile`) não tocam nada do cenário que falha.
