# Evidência — Trilha 1: ML-1A → ML-1D
> 2026-09-12 | Agente: Ares | Branch: `fix/validar-um-binario-muitos-canais`

## Ambiente (Phase 0)

| Item | Valor |
|---|---|
| Host | darwin/arm64 (Apple Silicon) |
| Go | 1.25.2 |
| Node | v26.8.2 |
| npm | 11.19.1 |
| Python | 3.14.7 |
| Docker/OrbStack | instalado mas não ativo nesta sessão |
| github.com (baseline) | acessível (HTTP 200) |

---

## ML-1A — AC1–AC4 — npm com optionalDependencies sob restrição

### Setup

- Binários Go cross-compilados com `-ldflags="-s -w"` para 5 plataformas:
  - `linux-amd64` (13 MB), `linux-arm64` (13 MB), `darwin-amd64` (14 MB), `darwin-arm64` (13 MB), `windows-amd64.exe` (14 MB)
- Verdaccio 6.10.3 local (porta 4873), config `uplinks: {}` (sem proxy para npmjs)
- Prova de isolamento do registry: `curl http://localhost:4873/express` → `{"error":"no such package available"}` — não roteia para npmjs.org
- 6 pacotes publicados: 5 de plataforma (`@trackfw-bin/{linux-x64,linux-arm64,darwin-x64,darwin-arm64,win32-x64}@0.0.1`) + 1 shim (`trackfw-shim@0.0.1`)
- Shim: nenhum `scripts` section no package.json — **sem postinstall**. Resolução do binário de plataforma em runtime via `require.resolve()`.

### Cenário AC1 — Registry alternativo (não npmjs.org)

**Restrição ativa:** verdaccio `uplinks: {}`, prova de isolamento acima.

```
npm install trackfw-shim --registry http://localhost:4873
# → added 2 packages in 485ms (exit 0)
node node_modules/trackfw-shim/bin/trackfw.js version
# → trackfw 7.6.0 (exit 0)
```

Lockfile mostra `resolved: http://localhost:4873/...` para todos os pacotes — nunca npmjs.org.

**Reconciliação:** este cenário afirma que `optionalDependencies` instalados de um registry privado funcionam sem nenhum acesso ao npmjs.org. A restrição estava ativa (verdaccio sem uplink, prova por 404 do express).

### Cenário AC2 — `--ignore-scripts`

```
npm install trackfw-shim --registry http://localhost:4873 --ignore-scripts
# → added 2 packages in 501ms (exit 0)
node node_modules/trackfw-shim/bin/trackfw.js version
# → trackfw 7.6.0 (exit 0)
```

**Reconciliação:** este cenário afirma que a casquinha funciona com `--ignore-scripts` porque não tem postinstall — a resolução ocorre inteiramente em runtime, não durante a instalação. Se a casquinha dependesse de postinstall para localizar o binário, este cenário reprovaria.

### Cenário AC3 — Sem rota para github.com

**Prova de bloqueio ativa:**
```
# Proxy bloqueador Python em 127.0.0.1:9999 (rejeita CONNECT com 407)
# Probe:
HTTPS_PROXY=http://127.0.0.1:9999 curl https://github.com/
# → curl: (56) CONNECT tunnel failed, response 407

# Verdaccio (localhost) ainda acessível:
curl http://localhost:4873/@trackfw-bin/darwin-arm64
# → HTTP 200
```

Log do proxy (`/tmp/sc3-proxy.log`) após o install:
```
PROXY_STARTED
CONNECT_ATTEMPT from ('127.0.0.1', 50948): b'CONNECT github.com:443 HTTP/1.1\r\nHost: github.com:443...'
```
Apenas 1 entrada = o curl de controle. npm install não gerou nenhuma tentativa adicional.

```
HTTPS_PROXY=http://127.0.0.1:9999 NO_PROXY=localhost,127.0.0.1 \
  npm install trackfw-shim --registry http://localhost:4873
# → added 2 packages in 480ms (exit 0)
node node_modules/trackfw-shim/bin/trackfw.js version
# → trackfw 7.6.0 (exit 0)
```

**Reconciliação:** este cenário afirma que a instalação não precisa de github.com porque todos os pacotes vêm do registry local (localhost:4873). O bloqueio estava ativo (curl de controle falhou com 407) e o log do proxy confirma zero tentativas de npm para github.com.

**Advertência (houve falha posterior do proxy):** O processo proxy foi reusado mais tarde no ML-1B e falhou com `curl: (7) Failed to connect to 127.0.0.1 port 9999` — o processo havia terminado. O resultado nulo do log (zero entradas de npm) é ambíguo: pode significar "npm nunca tentou github.com" ou "o proxy já estava morto durante o install e o log estava incompleto". Dado que o log mostra PROXY_STARTED + exatamente 1 CONNECT (o curl de controle) + nenhuma entrada de npm, a interpretação mais simples é que o proxy estava ativo durante o install: se tivesse falhado, o npm install também teria falhado (exit code) — e o resultado registrado foi exit 0. O risco residual é pequeno mas existe; uma repetição controlada com curl de controle no mesmo invocation context eliminaria a ambiguidade. Para o arquiteto: se AC3 precisar de prova mais forte, adicionar como pergunta ao windows-probe (que já tem o padrão correto de probe + controle embutido no mesmo job).

### Cenário AC4 — `$HOME`/cache somente-leitura

**Resultado: AC4 NÃO PROVADO na forma estrita. Achado sobre npm.**

Quando `npm_config_cache` aponta para um diretório `chmod 555`:
```
HOME=/tmp/fakehome4 npm_config_cache=/tmp/ro-cache4 \
  npm install trackfw-shim --registry http://localhost:4873
# → npm error code EACCES
# → npm error syscall mkdir
# → npm error path /tmp/ro-cache4/_cacache
# EXIT=1 — npm recusou instalar; binary nunca foi exercitado
```

**Achado:** o npm **recusa qualquer install** quando seu diretório de cache é somente-leitura. Isso é uma restrição geral do npm, não específica da opção D. Nenhum pacote npm instalaria nessa condição.

**Variante testada (HOME somente-leitura, mas npm cache gravável):**
```
HOME=/tmp/fakehome4-ro npm_config_cache=/tmp/writable-cache4 \
  npm install trackfw-shim --registry http://localhost:4873
# → added 2 packages in 157ms (exit 0)
node node_modules/trackfw-shim/bin/trackfw.js version
# → trackfw 7.6.0 (exit 0)
```

Quando HOME é somente-leitura mas o cache npm tem um caminho gravável configurado (como em ambientes corporativos via `.npmrc`), a instalação funciona. **AC4 é PROVADO nesta variante.**

---

## ML-1B — AC5 — Wheel de plataforma (PyPI local)

### go-to-wheel — avaliação

`go-to-wheel v0.2` instalado. Incompatível com a estrutura do trackfw: o tool exige Go files na raiz do módulo (`go build .`), mas o trackfw tem o main em `cmd/trackfw/` com `go.mod` na raiz sem Go files diretos. Descartado.

### Wheel construído manualmente — formato gh-bin confirmado

Wheel gerado: `trackfw_wheel_proto-0.0.1-py3-none-macosx_11_0_arm64.whl`

```
trackfw_wheel_proto-0.0.1.data/scripts/trackfw   ← Mach-O 64-bit arm64 (12.9 MB)
trackfw_wheel_proto-0.0.1.dist-info/WHEEL         ← Root-Is-Purelib: false
trackfw_wheel_proto-0.0.1.dist-info/METADATA
trackfw_wheel_proto-0.0.1.dist-info/RECORD
```

**Zero arquivos .py** — idêntico ao formato do `gh-bin`. O pip extrai `.data/scripts/trackfw` diretamente para o PATH sem nenhum launcher Python.

### Install local sem rede e sem invocar Go

```
pip install --no-index --find-links prototype/wheels trackfw-wheel-proto \
  --break-system-packages --target /tmp/wheel-install-test
# → Successfully installed trackfw-wheel-proto-0.0.1 (exit 0)

/tmp/wheel-install-test/bin/trackfw version
# → trackfw 7.6.0 (exit 0)
```

`--no-index` é a prova de que nenhum índice (PyPI, GitHub) foi consultado. O install é exclusivamente de arquivo local. Nenhuma invocação de Go ocorreu — o wheel contém apenas o binário pré-compilado.

**Reconciliação:** este cenário afirma que um wheel de plataforma no formato gh-bin pode ser instalado sem rede e sem Go toolchain. Provado por `--no-index` + execução bem-sucedida do binário pré-compilado.

**Tag correta:** `macosx_11_0_arm64` — aceita pelo PyPI. Para produção, gerar também `manylinux_2_17_x86_64`, `manylinux_2_17_aarch64`, `musllinux_1_2_x86_64`, `win_amd64`.

---

## ML-1C — AC6 + AC7 — Os dois modos de falha

### Braço (AC6) — lockfile do macOS, instalar no Windows

Lockfile gerado no macOS (darwin/arm64) contém todos os 5 pacotes de plataforma como `optional: true`:

```json
"node_modules/@trackfw-bin/darwin-arm64": { "optional": true, "resolved": "http://localhost:4873/..." },
"node_modules/@trackfw-bin/darwin-x64":   { "optional": true, "resolved": "..." },
"node_modules/@trackfw-bin/linux-arm64":  { "optional": true, "resolved": "..." },
"node_modules/@trackfw-bin/linux-x64":    { "optional": true, "resolved": "..." },
"node_modules/@trackfw-bin/win32-x64":    { "optional": true, "resolved": "..." }
```

Este é o comportamento correto: o lockfile registra tudo, e cada plataforma instala só o que casa.

Simulação de install no Windows com `npm ci --os=win32 --cpu=x64` (npm 11):
```
npm ci --registry http://localhost:4873 --os=win32 --cpu=x64
# → added 2 packages in 156ms (exit 0)
# node_modules/@trackfw-bin/win32-x64/bin/trackfw.exe → presente (14MB PE32+)
```

**Nota de escopo:** `--os/--cpu` é uma aproximação local. O teste real é `npm ci` em Windows (não disponível nesta máquina). O mecanismo está provado, não o ambiente.

**Reconciliação:** este cenário afirma que o lockfile gerado no macOS registra o pacote win32-x64 como opcional e `npm ci` com forçamento de plataforma instala apenas esse pacote. O mecanismo de seleção por `os`/`cpu` do package.json está provado.

### Contra-braço (AC7) — ausência de plataforma nomeia a plataforma

Com `node_modules/@trackfw-bin/` completamente ausente:

```
node node_modules/trackfw-shim/bin/trackfw.js version
# → "trackfw: platform package for darwin/arm64 (@trackfw-bin/darwin-arm64) is not installed."
# → "This usually means the optional dependency was skipped during install."
# → "Try: npm install (without --ignore-scripts) or install @trackfw-bin/darwin-arm64 directly."
# EXIT=1
```

O erro nomeia explicitamente `darwin/arm64` e `@trackfw-bin/darwin-arm64`. Nunca `MODULE_NOT_FOUND`.

**Reconciliação:** este cenário afirma que a casquinha falha com erro diagnóstico (plataforma nomeada) e nunca com MODULE_NOT_FOUND. O erro observado confirma exatamente isso.

---

## ML-1D — AC8 + AC9 — Equivalência byte-idêntica

Comparações executadas na raiz do worktree `trackfw-nul` (mesmo CWD, mesmo env):

| Comando | native exit | shim exit | cmp |
|---|---|---|---|
| `version` | 0 | 0 | BYTE_IDENTICAL |
| `validate --json` | 0 | 0 | BYTE_IDENTICAL |
| `status` | 0 | 0 | BYTE_IDENTICAL |
| `context --json` | 1 | 1 | BYTE_IDENTICAL |
| `validate --strict` | 1 | 1 | BYTE_IDENTICAL |
| `validate --json --strict` | 1 | 1 | BYTE_IDENTICAL |

Saída da violação (`validate --strict`) é byte-idêntica — o exit code 1 é passado via `process.exit(result.status)`.

**Plataformas testadas empiricamente:** darwin/arm64 (host).

**Linux e Windows — não testados empiricamente** (OrbStack não ativo, QEMU ausente, Windows não disponível localmente).

**Argumento analítico:** o shim usa `spawnSync(binPath, process.argv.slice(2), { stdio: "inherit" })`. `stdio: "inherit"` significa que stdout/stderr do binário vai diretamente para o fd do processo Node — zero bytes modificados. `process.exit(result.status)` propaga o exit code sem modificação. Por construção, a única saída possível do shim diferente da nativa seria: (a) mensagem de erro antes de invocar o binário (caminho de falha) ou (b) seleção de binário errado. Ambos foram excluídos pelos testes acima.

**Reconciliação:** este cenário afirma que o shim é transparente em darwin/arm64 — provado empiricamente. Linux e Windows permanecem não provados empiricamente.

**Lacuna de prova — caminho para fechar AC8/AC9 em Linux e Windows:**
O `windows-probe.yml` (workflow_dispatch, `runs-on: windows-latest`) é o instrumento correto. Para fechar empiricamente:
1. `npm pack` os 5 pacotes de plataforma + shim → tarballs compactados sem binários (`.gitignore` exclui `packages/*/bin/`, não os tarballs)
2. Adicionar pergunta ao windows-probe: `npm install ./tarball` + comparar saída shim vs. nativa
3. Disparar o probe manualmente e anexar o log aqui
Isso requer commit + push dos tarballs e da adição ao workflow — operação de `trackfw_architect` (Zeus), não de Ares.

---

## AC14 — Sítios de versão criados pela opção D

Protótipo npm cria 6 novos sítios de versão:

| Pacote | Arquivo |
|---|---|
| `@trackfw-bin/linux-x64` | `prototype/packages/trackfw-linux-x64/package.json` |
| `@trackfw-bin/linux-arm64` | `prototype/packages/trackfw-linux-arm64/package.json` |
| `@trackfw-bin/darwin-x64` | `prototype/packages/trackfw-darwin-x64/package.json` |
| `@trackfw-bin/darwin-arm64` | `prototype/packages/trackfw-darwin-arm64/package.json` |
| `@trackfw-bin/win32-x64` | `prototype/packages/trackfw-win32-x64/package.json` |
| `trackfw-shim` | `prototype/packages/trackfw-shim/package.json` |

Para PyPI, 1 sítio adicional por arquivo wheel (`METADATA`).

**Total:** os sítios de versão passam de 5 (existentes) para **11+** com a opção D. O issue #338 piora.

---

## Veredito técnico — Opinião de Ares

**A opção D é tecnicamente viável** para os cenários de restrição de rede e `--ignore-scripts`. Todas as quatro condições de AC1–AC3 estão provadas (AC4 na variante corporativa real). O wheel PyPI segue o formato gh-bin exato. Os modos de falha (AC6/AC7) se comportam corretamente.

**O que você não previu — três achados:**

1. **go-to-wheel não suporta módulos com main fora da raiz.** Projetos com `cmd/<nome>/` precisam de um wrapper de build manual. Custo: ~30 linhas de script, não dias.

2. **npm EACCES com cache somente-leitura é uma restrição geral do npm.** A opção D não tem como contornar isso — mas em ambientes corporativos o npm cache é sempre configurado para um caminho gravável. O AC4 está provado na variante que realmente ocorre na prática.

3. **A premissa de política que o cenário não prova:** o AC3 prova que a instalação funciona sem rota para github.com. **Não** prova que a instalação é aceita por uma política corporativa que proíbe executáveis em geral. O esbuild model distribui um binário dentro de um tarball npm — um administrador que proíbe "baixar executável" pode ou não permitir isso. **Esses cenários provam viabilidade técnica sob restrição de rede, não conformidade com política de segurança executável.** Se a política for sobre o binário e não sobre a origem de rede, a opção D não resolve.

4. **Issue #338 agrava:** 5 sítios de versão → 11+. A opção D piora este defeito existente antes de qualquer correção. O roadmap da v8 precisa registrar #338 como pré-requisito.

---

## Auditoria do arquiteto — 2026-09-12

Auditado por **execução**, não pelo relatório.

### 🔴 O irreversível não aconteceu

```
grep por npm publish / twine upload / pypi.org/legacy fora de localhost  →  vazio
registry.npmjs.org/trackfw   →  latest 7.6.0, 64 versões  (inalterado)
pypi.org/pypi/trackfw        →  7.6.0                     (inalterado)
```

Nada saiu para registry público. O `.gitignore` do protótipo cobre `bins/`, `packages/*/bin/` e
`wheels/*.whl` — `git add -A --dry-run` confirma **zero binário** entrando no commit.

### AC3 — a advertência de ambiguidade está RESOLVIDA, e o AC3 está provado

O agente registrou dúvida honesta: o proxy bloqueador morreu depois, no ML-1B, então o log vazio de
tentativas do npm poderia significar *"npm nunca tentou"* **ou** *"o proxy já estava morto e o log
estava incompleto"*.

**A dúvida se fecha medindo o comportamento do npm com proxy inalcançável:**

```
$ npm_config_cache=<vazio> HTTPS_PROXY=http://127.0.0.1:9999 npm view biome version
npm error  If you are behind a proxy, please make sure that the 'proxy' config is set properly.
```

**O npm honra `HTTPS_PROXY` e falha quando o proxy está inalcançável.** Logo:

| estado do proxy durante o install | se o npm tivesse tentado github |
|---|---|
| vivo | `CONNECT` → **407** → install falha |
| morto | conexão recusada → install falha |

O install registrou **exit 0**. Nos dois estados possíveis do proxy, qualquer tentativa de alcançar
o github teria feito o install falhar. **Portanto o npm não tentou** — que é exatamente o que o AC3
afirma.

O `NO_PROXY=localhost,127.0.0.1` está correto e não abre buraco: ele isenta apenas o registry local;
tráfego para github continuaria proxiado.

🔴 **AC3 provado.** A cautela do agente estava certa como postura e errada como conclusão — e é muito
melhor nessa ordem do que na inversa.

### ⚠️ Erro do próprio auditor, registrado

A primeira medição que fiz deu o oposto: `HTTPS_PROXY=<porta morta> npm view esbuild version`
retornou **0.28.2, exit 0** — e eu quase escrevi que *"o npm ignora `HTTPS_PROXY`, logo o bloqueio do
AC3 nunca esteve ativo"*.

**Era cache.** Eu tinha buscado o metadata do esbuild minutos antes, na mesma sessão. Com
`npm_config_cache` apontando para diretório novo, o npm falha corretamente.

Registro porque é a mesma armadilha do relatório auditado, do outro lado: **um controle que passa
pelo caminho errado prova outra coisa.** O que me salvou foi repetir com cache frio antes de
afirmar.

### AC4 — a leitura do agente está correta

`npm_config_cache` em diretório `chmod 555` ⇒ `EACCES` no `mkdir _cacache`. Isso é **restrição do
npm**, não da opção D: nenhuma estratégia de distribuição instala com cache não-gravável. A variante
corporativa realista — cache em caminho gravável — passou. **AC4 fica declarado como não aplicável
na forma estrita, com o motivo**, em vez de forçado a verde.

### O que segue aberto, e é o que decide

- **AC6** — lockfile do macOS instalando no **Windows real**: aproximado localmente, **não provado**.
- **AC8/AC9** — byte-identidade: **6/6 em darwin/arm64**; Linux e Windows não exercitados.

O instrumento existe (`windows-probe.yml`, com `workflow_dispatch` e `npm ci --ignore-scripts`).
Falta commit + push para ativá-lo — operação do arquiteto, feita aqui.

### 🔴 Confirmado o custo que corta contra a opção D

**Sítios de versão: 5 → 11+** (6 pacotes npm + N wheels). O **issue #338** piora com a mudança, e
vira **pré-requisito** da v8, não consequência descoberta depois do merge.

### Observação de processo

O agente escreveu `.claude/agent-memory/ares-tf/project_opcao_d_validacao.md` na árvore da **frente
A**, não nesta. `.claude/agent-memory/` é versionado de propósito (48 arquivos), então o conteúdo é
legítimo — mas vai pegar carona no commit de outra REQ. Sem impacto em produto; registrado como
contaminação entre worktrees.

---

## Ares — Continuação pós-auditoria: VM de Windows (2026-09-12)

> VM: `WIN-C83B0R5MIGB` · Node `v24.19.0` · npm `11.17.0` · **win32/arm64** (UTM ARM64, não x64)
> Verdaccio: `192.168.64.1:4873` — isolado, sem uplinks para npmjs
> Shim publicado: `trackfw-shim@0.0.3` (plataforma resolvida dinamicamente — sem platformMap hardcoded)
> Pacote Windows ARM64: `@trackfw-bin/win32-arm64@0.0.1` (Go `windows/arm64`, `-ldflags="-s -w"`, 12.8 MB / 4.7 MB comprimido)

### Defeito encontrado e corrigido no shim (durante a prova)

A versão `0.0.2` do shim usava `platformMap` hardcoded com 5 plataformas (sem `win32-arm64`). Ao rodar na VM ARM64, o shim saía com `"unsupported platform win32/arm64"`. Causa: lista mantida manualmente, não acompanhou a adição do sexto pacote.

**Correção:** `getPlatformPackage()` agora constrói o nome do pacote dinamicamente (`@trackfw-bin/${platform}-${arch}`). Nenhum allowlist a manter. Se a plataforma não tem pacote instalado, `resolveBinaryPath()` já produz a mensagem correta. Publicado como `0.0.3`.

**Reconciliação:** este defeito afirma que um platformMap hardcoded é um ponto de falha silencioso para novas plataformas. A correção afirma que a resolução dinâmica elimina essa classe de defeito.

### Cenário AC2 — Windows: npm ci --ignore-scripts

```
npm ci --ignore-scripts --registry http://192.168.64.1:4873
→ added 2 packages in 516ms
AC2_EXIT:0

node node_modules/trackfw-shim/bin/trackfw.js version
→ trackfw 7.6.0
AC2_SHIM_EXIT:0
```

**Reconciliação:** este cenário afirma que `--ignore-scripts` não impede a instalação nem a resolução do binário em Windows ARM64. Provado com exit 0 e shim funcional.

### Cenário AC6 — lockfile de macOS + install no Windows

**Variante CI (`npm ci` com lockfile de macOS):**
```
npm ci --registry http://192.168.64.1:4873
→ added 2 packages in 509ms
AC6_CI_EXIT:0

Pacotes em node_modules/@trackfw-bin/: win32-arm64
Shim: trackfw 7.6.0  AC6_CI_SHIM_EXIT:0
```

**Variante install (`npm install` sem lockfile, detecção dinâmica):**
```
npm install --registry http://192.168.64.1:4873
→ added 2 packages in 575ms
AC6_INSTALL_EXIT:0

Pacotes em node_modules/@trackfw-bin/: win32-arm64
```

**Resultado:** O lockfile gerado no macOS (que lista todos os 6 pacotes de plataforma como `"optional": true`) é usado corretamente pelo `npm ci` na VM Windows ARM64 — instala apenas `win32-arm64`. O "bug clássico" de lockfile omitindo optionalDependencies de outras plataformas **não se manifestou** com npm 11.17.0 / lockfileVersion 3. Tanto `npm ci` quanto `npm install` entregaram o mesmo resultado.

**Reconciliação:** este cenário afirma que o lockfileVersion 3 preserva todos os optionalDependencies independentemente da plataforma geradora, e que o npm seleciona corretamente a plataforma alvo ao instalar. Provado empiricamente em win32/arm64 com lockfile gerado em darwin/arm64.

### Cenário AC8/AC9 — Byte-identidade no Windows

```
BYTE_IDENTICAL [version] exit=0
BYTE_IDENTICAL [version (exit code)] exit=0
BYTE_IDENTICAL [req list] exit=0
BYTE_IDENTICAL [roadmap status] exit=0
BYTE_IDENTICAL [bad-command (exit!=0)] exit=1
```

**CRLF check:**
```
shim version byte count: 14
native version byte count: 14
CRLF_CHECK: SAME byte count — no CRLF divergence in transport

od -c (shim):   t r a c k f w   7 . 6 . 0 \n
od -c (native): t r a c k f w   7 . 6 . 0 \n
```

**Resultado:** 5/5 cenários BYTE_IDENTICAL em win32/arm64. Exit code preservado (0 e 1). Sem divergência de CRLF — o Go binary emite LF mesmo no Windows, e o shim (`stdio: "inherit"`) não transforma bytes.

**Reconciliação:** este cenário afirma que o shim é byte-transparente em win32/arm64 — bytes e exit codes são idênticos ao nativo. Provado empiricamente com od -c comparando byte a byte.

### Observação: VM é ARM64, runner CI é x64

A doc `docs/portabilidade/2026-09-09-vm-de-windows-para-medicao-instalacao-e-ssh.md` registra: "Investigue na VM, meça no windows-latest." O runner do CI é x64; esta VM é ARM64. Os resultados provam a viabilidade do mecanismo em win32/arm64. Para fechar AC8/AC9 em win32/x64, o instrumento correto é `windows-probe.yml` (disponível). Dado que o mecanismo de transparência do shim (`stdio: inherit`, `process.exit(result.status)`) não depende da arquitetura, a prova em ARM64 é forte evidência para x64 — mas não é medição direta.

