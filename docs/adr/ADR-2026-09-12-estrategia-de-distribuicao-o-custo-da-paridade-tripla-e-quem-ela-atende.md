---
name: ADR-2026-09-12-estrategia-de-distribuicao-o-custo-da-paridade-tripla-e-quem-ela-atende
status: Proposed
date: 2026-09-12
---

# ADR: Estratégia de distribuição — o custo da paridade tripla e quem ela atende

> **Status: Proposed.** Esta ADR **não decide**; ela congela a medição e a análise de 2026-09-12
> para que a decisão da v8/v9 seja tomada com o trabalho pronto em vez de refeito. A decisão exige
> dados de adoção que **hoje não existem** — ver "Quando decidir".

## Contexto

O trackfw **nasceu em Go**. Os CLIs de Node.js e Python foram criados depois, **por convenção**,
para atender organizações cuja política de segurança **proíbe baixar executáveis** — `npm` e `pip`
já são liberados nelas, binário solto não. O Go é a expressão da verdade
(`CLAUDE.md`, `docs/cli-parity.md`).

Em 2026-09-12 o KG levantou a hipótese de **descontinuar Node.js e Python na v8 ou v9** e, no lugar,
criar um **instalador para Windows**.

## O custo, medido

```
Go       33.267 linhas  (internal/ + cmd/, sem testes)
Node     26.272 linhas
Python   27.472 linhas
                        ─────────────────────────────
                        53.744 linhas de reimplementação

gates    31 de 61 existem exclusivamente para vigiar paridade
```

🔴 **Metade do arsenal de gates existe para vigiar uma duplicação tripla.** E a paridade não é só
cara: na campanha de defeitos de 2026-09, ela foi a **principal origem** dos achados — agente
resolvido no runtime errado, regra presente em dois dos três, artefato de `init` divergente, e
gates de paridade que comparavam a coisa errada.

## Quem ela atende, medido

```
npm                    1.810 downloads/mês   (25 de 30 dias com download)
PyPI                   1.762 downloads/mês
tap Homebrew (14 d)    23 clones · 17 únicos · 0 views    ← clone sem view ≈ brew install
repo principal (14 d)  6.435 clones · 380 únicos          ← o próprio CI
stars 6 · forks 1 · watchers 0

binário Windows nos releases:  0 · 0 · 2 · 0 · 0 · 0 downloads
```

**Issues abertos no projeto: 39 de um único autor externo, 2 do mantenedor. Mais ninguém.**

E esse autor, pelos próprios issues: **Windows 11, Git for Windows, Python 3.12.4, Node do PATH,
trabalhando de um fork em `main`.** Ele **compila do fonte** — não consome npm, pip nem brew.

### 🔴 O que a medição sustenta, e o que não sustenta

**Não sustenta** que npm e pip carreguem adoção: eles carregam **downloads**, e sem um único humano
identificável atrás deles, o mais provável é CI, mirrors e bots. *(Esta ADR corrige uma afirmação
anterior do arquiteto, que tratou os downloads como adoção.)*

**Sustenta** que o único sinal humano plausível é o tap do Homebrew — 17 clones únicos em 14 dias.

**Não sustenta** que a audiência de "empresas que não podem baixar executável" exista: ela não
aparece em nenhum canal. 🔴 **Mas ausência de evidência não é evidência de ausência** — esse público,
por definição, é corporativo e silencioso, e o projeto tem 6 stars: está em pré-adoção, e os canais
foram construídos para um público futuro.

**O que fica estabelecido:** hoje pagamos 53.744 linhas e 31 gates por um público de que **não temos
nenhuma evidência**.

## Análise das opções

### A — Descontinuar Node e Python, criar instalador de Windows (a proposta original)

🔴 **O instalador não substitui o que os dois canais atendem.** Um `.msi` ou `.exe` **é um executável
baixado** — não atende a restrição que originou npm e pip. A troca não migra esse público: perde-o.

E o instalador serviria um canal com **zero demanda medida**: o tarball de Windows tem 0–2 downloads
em toda a história do projeto. Duas leituras, e as duas desaconselham agora: ou não há usuários de
Windows, ou eles já entram por outro caminho.

**Ressalva a favor:** o único usuário externo real **é** de Windows. Mas ele compila do fonte porque
contribui, não porque instalar é difícil.

### B — Manter os três como estão

Preserva opcionalidade e continua pagando 53.744 linhas e 31 gates por mês, com a paridade seguindo
como principal fábrica de defeito.

### C — Um comportamento, três canais: Go compilado também para WebAssembly

```
   UM código-fonte Go
        ├── go build ──────────────────────► binário nativo   (brew · install.sh)
        └── GOOS=wasip1 GOARCH=wasm ───────► trackfw.wasm
                                              ├── pacote npm  (casquinha JS)
                                              └── pacote pip  (casquinha Python)
```

🔴 **O binário nativo NÃO usa o `.wasm`.** Ele continua sendo o produto principal — mais rápido, sem
dependência de runtime. O `.wasm` é um **segundo artefato de build do mesmo código**.

A "casquinha fina" substitui as 26.272 linhas do Node e as 27.472 do Python por algumas centenas que
só fazem: ler `argv`, carregar o `.wasm` do pacote, ligar stdio e os diretórios do projeto, devolver
o exit code. **Nenhuma regra de negócio.** Os 31 gates de paridade não passam a passar — eles
**deixam de ter objeto**, porque não existe mais uma segunda implementação para comparar.

**Ressalvas medidas:**

1. O alvo é `wasip1`, não `js` — WASI dá argv, env e sistema de arquivos. Mas o acesso é por
   *preopen*: um CLI que anda por caminhos arbitrários (`adr_dirs` com `~/...`) exige cuidado real.
2. Node tem WASI embutido (`node:wasi`), com status experimental conforme a versão.
3. 🔴 **Python não tem runtime WASM embutido.** Precisaria de `wasmtime`, **cujas wheels trazem
   binário nativo dentro** — ou seja, o canal pip passaria a baixar um executável, que é
   **exatamente a restrição que justifica ele existir**.
4. Binário Go em wasm é grande (alguns MB) e mais lento que nativo; irrelevante para esta carga,
   mas não é grátis.

### 🔴 A assimetria que a ressalva 3 revela

Se o único motivo do canal Python é *"não pode baixar executável"*, então o Python **não tem
caminho**: manter a reimplementação é caro demais, e migrar para WASM viola a própria premissa.
O Node sobrevive à migração; o Python não.

**Se algo for descontinuado, o candidato natural é o Python — não os dois.**

## Decisão

**Nenhuma, por ora.** Registram-se três posições:

1. 🔴 **Não descontinuar nada com os dados atuais.** Seis stars e um usuário externo é pré-adoção;
   decidir distribuição aqui é decidir no ruído, e é irreversível na prática.
2. 🔴 **Não criar o instalador de Windows agora.** Ele não atende a restrição que originou os canais
   e serve um canal com zero demanda medida.
3. **Parar de fazer a dívida crescer.** Toda feature nova continua exigindo os três (a regra dura
   segue valendo), mas **feature nova deve ser avaliada contra a opção C antes de ser triplicada**.

## Quando decidir

Esta ADR vira `Accepted` com decisão real quando existir **pelo menos um** destes:

- um usuário externo identificável instalando por npm ou pip — hoje: **zero**;
- evidência direta da restrição corporativa (issue, e-mail, conversa) — hoje: **zero**;
- adoção do tap acima de ~100 únicos/mês, indicando que o canal nativo basta — hoje: ~34/mês
  extrapolado;
- um protótipo de C medindo tamanho do `.wasm`, viabilidade do preopen para `adr_dirs` e se o
  `wasmtime` é aceitável no pip.

**O quarto item é o único que depende só de nós, e é o de maior valor:** ele responde se a opção C é
viável **antes** de precisarmos escolher, e custa um protótipo descartável.

## Consequências

- A regra dura de paridade **continua valendo integralmente** até esta ADR ser aceita com outra
  decisão. Nada muda no dia a dia.
- Registra-se que a medição foi feita e o que ela mostra, para a discussão da v8 **não recomeçar do
  zero**.
- 🔴 Registra-se o erro de leitura corrigido: downloads de registro **não são** adoção enquanto não
  houver um humano identificável atrás deles.

## Limites desta medição

- Downloads de npm e PyPI são inflados por CI, mirrors e bots, e **não sei separar**. Sustentam
  comparação relativa, não contagem de usuários.
- Tráfego do tap são 14 dias — janela curta, e a API do GitHub não dá mais.
- `brew` não publica analytics para tap de terceiro; clones do tap são **proxy**, não instalações.
- Não medi Windows Package Manager, Scoop, Chocolatey nem AUR — nenhum tem pacote nosso, mas não
  verifiquei se alguém empacotou por conta.
