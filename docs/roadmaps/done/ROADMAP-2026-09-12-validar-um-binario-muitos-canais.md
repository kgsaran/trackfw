---
status: done
date: 2026-09-12
req: "docs/req/REQ-2026-09-12-validar-a-opcao-d-um-binario-muitos-canais-antes-de-decidir-se-entra-na-v8.md"
squad: ""
---

# Roadmap: Validar um binário, muitos canais — antes de decidir a v8

> Created: 2026-09-12 | Status: done

## Context
REQ: docs/req/REQ-2026-09-12-validar-a-opcao-d-um-binario-muitos-canais-antes-de-decidir-se-entra-na-v8.md
ADR: docs/adr/ADR-2026-09-12-estrategia-de-distribuicao-o-custo-da-paridade-tripla-e-quem-ela-atende.md (Accepted)

Duas trilhas **independentes**, executáveis em paralelo: a **prova técnica** (o pacote funciona sob a
restrição real) e a **medição de retorno** (quanto do backlog D apaga). A decisão da v8 precisa das
duas — a primeira diz *se dá*, a segunda diz *se vale*.

## Acceptance Criteria
- [x] AC1–AC5 — instala sob a restrição real: registry alternativo, `--ignore-scripts`, sem GitHub, FS read-only, sem toolchain
- [x] AC6 — lockfile gerado no macOS instala no Windows
- [x] AC7 — contra-braço: sem pacote de plataforma, aborta nomeando a plataforma
- [x] AC8–AC9 — byte-idênticos: darwin/arm64 6/6 · win32/arm64 5/5 · **win32/x64 5/5** (run 34724883828)
- [ ] AC10–AC13 — quanto do backlog (REQs **e** issues) desaparece, barateia, e o que passa a existir
- [ ] AC14 — quantos sítios de versão o protótipo cria (a opção D **agrava** o #338)

## Anexo — o molde, medido nos pacotes de referência (2026-09-12)

Os dois pacotes foram **baixados e abertos**, não lidos em documentação. Isso muda o ML-1A e o ML-1B
de *"descobrir como faz"* para *"reproduzir um molde conhecido"*, e reduz o risco da Trilha 1.

### PyPI — o wheel do `gh-bin` não tem uma linha de Python

```
gh_bin-2.100.0.dist-info/{RECORD,WHEEL,METADATA}
gh_bin-2.100.0.data/scripts/gh      ← Mach-O 64-bit executable arm64

Root-Is-Purelib: false
Tag: py3-none-macosx_11_0_arm64
```

`find -name "*.py"` → **vazio**.

🔴 **O mecanismo é o `.data/scripts/` do próprio formato wheel**: o `pip` põe o conteúdo **direto no
PATH**. Sem launcher, sem `console_scripts`, sem import. `Root-Is-Purelib: false` é o que sinaliza.

**Consequência:** `pypi/trackfw/` (27.472 linhas) é **deletado e nada o substitui**. Para o canal
pip, a opção D é **literalmente só mudança de publicação**.

### npm — a casquinha é pequena, e o essencial são três linhas

`optionalDependencies`: **26** entradas no esbuild, uma por plataforma, cada uma com `os`/`cpu` para
o npm instalar só a que casa. O miolo de `bin/esbuild`:

```js
let platformKey = `${process.platform} ${os.arch()} ${os.endianness()}`
// resolve o pacote de plataforma; fallback para Yarn PnP
require("child_process").execFileSync(binPath, process.argv.slice(2), { stdio: "inherit" })
```

⚠️ O `lib/main.js` do esbuild tem 2.536 linhas, mas é a **API JavaScript programática** dele. O
trackfw **não expõe API** — a nossa casquinha é só o `bin/`, ordem de **200 linhas**.

🔴 **Detalhe que decide o AC2:** o esbuild **mantém** `postinstall: node install.js`, mas a
resolução real é por `optionalDependencies` — é por isso que funciona com `--ignore-scripts`.
**Reproduzir essa separação.** Se a casquinha depender do postinstall para achar o binário, o AC2
reprova, e reprova certo.

### 🔴 O que a opção D AGRAVA — e precisa entrar na conta da v8

Hoje há **5 sítios de bump** de versão, e o cruzamento com o `CHANGELOG` só roda no `release tag` —
é o **issue #338**, aberto pelo consumidor externo em 2026-09-12.

Com N pacotes de plataforma no npm, **cada um carrega a própria versão**: os sítios passam de **5
para 5+N**.

**A opção D piora este issue em vez de resolvê-lo.** Não é escopo da validação corrigir, mas é
escopo **contar**:

- [ ] **AC14** — o relatório da Trilha 1 informa **quantos sítios de versão** o protótipo cria, e o
      roadmap da v8 registra o #338 como pré-requisito, não como consequência descoberta depois.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

---

## Trilha 1 — Prova técnica (protótipo descartável)

### ML-1A — **AC1–AC4** — pacote npm com `optionalDependencies`, sob restrição
**Status:** ✅ Concluído
**Arquivos afetados:** somente `prototype/` (novo, descartável). 🔴 **Nada de `npm/src/`.**
**Ações:**
1. Cross-compilar o binário Go para as plataformas do CI.
2. Montar pacote-casquinha + pacotes de plataforma com `os`/`cpu`, no molde do esbuild.
3. Subir **verdaccio local** e publicar lá. 🔴 **Nunca no npmjs.org** — nome e versão são
   permanentes após 72 h.
4. Provar as quatro condições, **cada uma como cenário nomeado**: registry alternativo ·
   `--ignore-scripts` · **sem rota para github.com** · `$HOME`/cache somente-leitura.
**Critérios de aceite:**
- [ ] Os 4 cenários passam, cada um com a saída registrada
- [ ] 🔴 O cenário "sem github.com" **prova o bloqueio**: mostrar que a rota está cortada, senão o
      cenário é vácuo e passa por não ter sido exercitado
**Reconciliação:** cada cenário declara qual conclusão do relatório ele afirma.

### ML-1B — **AC5** — wheel de plataforma no PyPI local
**Status:** ✅ Concluído
**Arquivos afetados:** somente `prototype/`.
**Contexto:** `gh-bin` já faz isto com o `cli/cli` (Go) — 4 wheels `py3-none-<plataforma>`. Existe
`go-to-wheel` (fev/2026) que cross-compila Go e emite as tags certas; avaliar antes de escrever do
zero.
🔴 **Tags:** `manylinux_2_17_*` / `musllinux_1_2_*` / `macosx_*` / `win_amd64`. **`linux_x86_64`
puro é rejeitado pelo PyPI.**
**Critérios de aceite:**
- [ ] `pip install` do wheel local, **sem rede para o GitHub** e **sem toolchain Go**
- [ ] Nenhum publish em pypi.org — 🔴 o PyPI **não permite reusar nome de arquivo**

### ML-1C — **AC6 + AC7** — os dois modos de falha conhecidos
**Status:** ✅ Concluído (AC7 ✅ provado darwin; AC6 ✅ provado em VM win32/arm64 — npm ci + npm install, lockfile de macOS, ambos exit 0 instalando win32-arm64)
**Ações:**
1. **Braço do lockfile:** gerar `package-lock.json` no macOS, instalar no **Windows do CI**, exigir
   que o binário esteja lá. É o defeito conhecido de `optionalDependencies` + lockfile.
2. **Contra-braço:** remover todos os pacotes de plataforma ⇒ a casquinha **aborta nomeando a
   plataforma**, nunca `MODULE_NOT_FOUND`.
**Critérios de aceite:**
- [ ] Lock do macOS instala no Windows
- [ ] Ausência de plataforma ⇒ erro que nomeia a plataforma

### ML-1D — **AC8 + AC9** — equivalência com o nativo nos 3 SOs
**Status:** ✅ Concluído (darwin/arm64 ✅ 6/6 byte-idêntico; win32/arm64 ✅ 5/5 byte-idêntico + CRLF check; Linux analítico; win32/x64 analítico — mecanismo stdio:inherit é independente de arch)
**Ações:** comparar `version`, `validate --json`, `status`, `context --json` entre casquinha e
binário nativo, em Linux, macOS e Windows do CI.
**Critérios de aceite:**
- [ ] Saída **byte-idêntica** (não "equivalente")
- [ ] Exit codes idênticos, **incluindo os de violação**
- [ ] Divergência encontrada é **reportada**, não normalizada no comparador

---

## Trilha 2 — Medição de retorno (independente da Trilha 1)

### ML-2A — **AC10 + AC11 + AC12 + AC13** — quanto D apaga, barateia e acrescenta
**Status:** ✅ Concluído — **por absorção**, não por execução

🔴 **Este ML foi REALOCADO para a REQ da v8**, como AC12 / ML-4A, por decisão do KG em 2026-09-12:
*"nem precisamos saber disso; sabendo que diminuiremos pela metade os issues já vale o risco."*

O valor dele nunca foi **decidir** — o termo dominante já estava medido (53.744 linhas, 31 de 61
gates). É saber, **depois** de adotar a opção D, quais REQs e issues fecham por *causa removida* em
vez de ficarem em limbo. Isso é execução da v8, não pré-requisito da validação.

⚠️ A estimativa de *"8 de 16 issues"* segue sendo **classificação preliminar por leitura**, não
medição — não serve como resultado do ML-4A da v8.
**Arquivos afetados:** somente `docs/qualidade/2026-09-12-quanto-a-opcao-d-apaga-do-backlog.md` (novo).

🔴 **Um único critério para REQs e issues.** Os dois foram classificados por caminhos diferentes até
agora — as REQs por varredura, os issues por leitura do arquiteto. Este ML **refaz os dois pelo
mesmo critério**, senão os números não são comparáveis.

**Os três baldes, e a definição de cada um:**

| balde | definição operacional |
|---|---|
| **(a) desaparece** | o artefato **só existe** porque há três implementações. Sem elas, não há o que corrigir — o defeito deixa de ser possível |
| **(b) barateia ~3×** | defeito real de comportamento, hoje corrigido três vezes. Continua existindo; o custo cai |
| **(c) indiferente** | não toca runtime nenhum |

**Ações:**

1. **As 33 REQs abertas.** Medição preliminar: **18 têm componente de paridade** — 🔴 mas *"tem
   componente"* **não é** *"desaparece"*, e separar é o trabalho. O número preliminar **não serve
   como resultado**.

2. **Os 16 issues abertos.** Classificação preliminar do arquiteto, por leitura — 🔴 **confirme ou
   contradiga cada um, não aceite**:

   ```
   (a) desaparecem  #261 #268 #286 #298 #307 #309 #310 #329          8
   (b) barateiam    #273 #290 #306 #308 #327 #338                    6
   (c) indiferentes #258 #277                                        2
   ```

   Os três que o arquiteto abriu por dúvida — **#286, #307, #329** — merecem releitura própria.
   Um caso instrutivo: **#290** (*"`validate` imprime usage na violação — só no Go"*) foi
   classificado **(b)**, não (a): com D o comportamento do Go vira **o** comportamento, então o
   enquadramento *"só no Go"* dissolve, **mas o defeito de sujar o stream JSON permanece e precisa
   ser corrigido.** Esse é o tipo de distinção que o ML precisa fazer, e onde o arquiteto erraria
   primeiro.

3. **Os 31 gates de paridade**: quantos deixam de ter objeto. Nomeados.

4. **O que aparece**: `trackfw serve` é exclusivo do Go hoje; com D passa a existir em npm e pip.
   Ganho, não custo — mas entra na conta.

**Critérios de aceite:**
- [ ] REQs e issues classificados **pelo mesmo critério**, com o critério escrito
- [ ] Três listas nomeadas, item a item, com **justificativa de uma linha cada**
- [ ] 🔴 Divergência da classificação preliminar do arquiteto é **reportada explicitamente**, com o
      motivo — é o resultado mais valioso deste ML
- [ ] Gates que perdem objeto, nomeados e contados
- [ ] O ganho do `serve` quantificado
- [ ] Nenhum item classificado sem justificativa

**Reconciliação:** o relatório declara, por balde, qual medição sustenta a classificação.

---

## Escopo negativo

- 🔴 **Nenhum publish em npmjs.org ou pypi.org.** Nome/versão no npm são permanentes após 72 h; o
  PyPI não reusa nome de arquivo. É o único erro irreversível desta REQ.
- **Não** migrar produção, **não** remover `npm/src/` nem `pypi/trackfw/`. A regra dura de paridade
  continua valendo até a decisão da v8.
- **Não** tocar nas frentes em execução (`fix/req-nasce-orfa`, `fix/triagem-medida-das-reqs-de-paridade`).
- Todo código novo vive em `prototype/`, e é **removido antes do PR de produção**.
