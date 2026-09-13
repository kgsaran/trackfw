---
status: Open
date: 2026-09-12
author: ""
adr: ""
roadmap: ""
---

# REQ: validar a opção D — um binário, muitos canais — antes de decidir se entra na v8

> Date: 2026-09-12 | Status: Open

## Contexto

`ADR-2026-09-12-estrategia-de-distribuicao` foi **aceita**: a direção é a **opção D** — uma
implementação em Go, distribuída por muitos canais, com o binário nativo **dentro** do pacote, em
vez de reimplementada em Node e Python.

O **quando** não está decidido. Esta REQ produz a evidência para responder *"entra na v8?"*.

## Prior art confirmado — o padrão existe e há precedente em Go nos dois canais

| ferramenta | escrita em | canal | mecanismo |
|---|---|---|---|
| **esbuild** | **Go** | npm | 26 `optionalDependencies` por plataforma |
| biome | Rust | npm | 8 `optionalDependencies` |
| **gh-bin** | **Go** (é o `cli/cli`) | **PyPI** | 4 wheels `py3-none-<plataforma>` |
| ruff · uv | Rust | PyPI | 17 e 18 wheels de plataforma |

Tags que o PyPI aceita, medidas no ruff: `manylinux_2_17_*`, `musllinux_1_2_*`, `macosx_*`,
`win32`/`win_amd64`/`win_arm64`. 🔴 **`linux_x86_64` puro é rejeitado** — Linux exige
`manylinux*`/`musllinux*`.

Existe ferramenta pronta: **`go-to-wheel`** (fev/2026) cross-compila um módulo Go e emite wheels com
as tags corretas.

## 🔴 O que esta validação precisa exercitar — a restrição, não o caminho feliz

`npm install` de um tarball local numa máquina de dev **não prova nada**. A premissa nunca foi *"um
binário roda"*; foi *"GitHub bloqueado, política proíbe baixar executável, postinstall desabilitado"*.

O esbuild nomeou as condições de vitória ao migrar de download-no-postinstall para
`optionalDependencies`, e elas viram critério aqui **literalmente**:

> *"registries customizados, proxies customizados, instalações offline, sistemas de arquivos
> somente-leitura, ou quando scripts de post-install estão desabilitados."*

## Acceptance Criteria

### Prova sob a restrição real

- [ ] **AC1** — instalar de um registry que **não** é o npmjs.org (verdaccio local, ou
      `npm --registry` apontando para mirror de arquivo).
- [ ] **AC2** — instalar com **`--ignore-scripts`** (postinstall desabilitado) e funcionar.
- [ ] **AC3** — 🔴 instalar **sem rota para `github.com`** — bloqueado no ambiente de teste — e a
      instalação resolver mesmo assim. **É o AC que representa o motivo de a REQ existir.**
- [ ] **AC4** — instalar com `$HOME`/cache **somente-leitura**.
- [ ] **AC5** — equivalente do pip: instalar wheel de plataforma sem rede para o GitHub, e sem
      compilador Go ou Rust na máquina.

### Os dois modos de falha conhecidos

- [ ] **AC6** — 🔴 **lockfile × `optionalDependencies`**: `package-lock.json` gerado no macOS pode
      **omitir** a dependência opcional de Windows, e o install no Windows fica sem binário. Braço:
      gerar o lock no macOS, instalar no **Windows do CI**, exigir que funcione.
      *(Contexto: o nosso `package-lock.json` está em 7.3.0 com o pacote em 7.6.0 — ver
      `REQ-2026-08-30-debitos-de-higiene`.)*
- [ ] **AC7** — **contra-braço**: se nenhum pacote de plataforma resolver, a casquinha **aborta
      nomeando a plataforma**. Nunca cair num `MODULE_NOT_FOUND` confuso.

### Equivalência

- [ ] **AC8** — a saída da casquinha é **byte-idêntica** à do binário nativo para um conjunto de
      comandos (`version`, `validate --json`, `status`, `context --json`), nos **três SOs** do CI.
- [ ] **AC9** — exit codes idênticos, incluindo os de violação.

### A medição que decide a v8

- [ ] **AC10** — 🔴 classificar as **33 REQs abertas** em três baldes, com a lista escrita:
      **(a) desaparecem** com D (o defeito é "existe num runtime e falta noutro");
      **(b) barateiam** ~3× (defeito real, hoje corrigido três vezes);
      **(c) indiferentes**.
      Medição preliminar: **18 das 33 têm componente de paridade** — mas "componente" não é
      "desaparece", e a diferença é o ponto do AC.
- [ ] **AC11** — mesmo exercício para os **31 gates de paridade**: quantos deixam de ter objeto.
- [ ] **AC12** — quantificar o que **aparece**: `trackfw serve` é exclusivo do Go hoje; com D ele
      passa a existir em npm e pip. Ganho, não custo — mas precisa estar na conta.

## 🔴 Escopo negativo — a coisa irreversível

- **NÃO publicar nos registries reais.** Nome e versão no npm são efetivamente permanentes após 72 h,
  e o PyPI **não permite reusar nome de arquivo**. Um publish de teste sob o nome `trackfw` é o único
  erro irreversível disponível nesta REQ. **Registry local e instalação de wheel local, apenas.**
- **Não** migrar nada de produção. Esta REQ produz um protótipo **descartável** e uma medição.
- **Não** remover `npm/src/` nem `pypi/trackfw/`. A regra dura de paridade **continua valendo** até a
  decisão da v8.
- **Não** mexer nas frentes em execução (`fix/req-nasce-orfa`, `fix/triagem-medida-das-reqs-de-paridade`).

## Break declarado, para não ser descoberto depois

O pacote npm hoje publica `files: ['bin/', 'src/']`. Trocar `src/` por casquinha + binário **quebra
quem faça `require('trackfw')`** como biblioteca. Risco baixo nesta adoção, mas é **break declarado
no CHANGELOG**, não surpresa.

## Linked ADR
ADR: docs/adr/ADR-2026-09-12-estrategia-de-distribuicao-o-custo-da-paridade-tripla-e-quem-ela-atende.md
