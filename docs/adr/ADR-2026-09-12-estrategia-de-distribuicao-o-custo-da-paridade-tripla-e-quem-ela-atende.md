---
name: ADR-2026-09-12-estrategia-de-distribuicao-o-custo-da-paridade-tripla-e-quem-ela-atende
status: Accepted
date: 2026-09-12
---

# ADR: Estratégia de distribuição — o custo da paridade tripla e quem ela atende

> **Status: Accepted** (KG, 2026-09-12). A direção está decidida: **opção D — um binário, muitos
> canais.** O que **não** está decidido é o *quando* — se entra na v8 —, e isso depende da validação
> descrita em "Quando decidir", que passou de gatilho passivo a **trabalho em execução**.
>
> As posições 1 e 2 (não descontinuar canal, não fazer o instalador de Windows) seguem valendo e
> foram **absorvidas** pela decisão: com D, não se descontinua canal porque manter passa a ser
> barato.

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

### 🔴 D — Um binário, muitos canais: o que o mercado de fato faz

**Levantado em 2026-09-12, depois das opções acima, e supera as três.**

Nenhum CLI comparável reimplementa a si mesmo em vários runtimes. O padrão é **uma implementação,
distribuída por muitos canais, com o binário nativo dentro do pacote**:

| ferramenta | escrita em | canal | mecanismo |
|---|---|---|---|
| **esbuild** | **Go** | npm | **26** `optionalDependencies` por plataforma |
| biome | Rust | npm | 8 `optionalDependencies` |
| ruff | Rust | PyPI | **17 wheels** `py3-none-<plataforma>` |
| uv | Rust | PyPI | 18 wheels de plataforma |
| gh · terraform · kubectl | Go | brew · winget · choco · scoop · apt · MSI | **nem npm nem pip** |

O `py3-none-*` do ruff diz tudo: **`none` é a tag de ABI de "sem código Python"** — o wheel é o
binário embrulhado. Ninguém reimplementou nada.

E **o esbuild é escrito em Go e distribuído principalmente por npm.** É o precedente exato do que
esta ADR discute.

#### 🔴 O modelo de wrapper não falhou — a nossa implementação dele falhou

O histórico do projeto registra que o CLI Python **era** um wrapper e foi abandonado porque
*"baixava o binário Go do GitHub — modelo que falha em ambientes corporativos com GitHub bloqueado"*.

**O defeito era baixar no `postinstall`, não embrulhar o binário.** O esbuild passou por exatamente
isto e migrou de download-no-postinstall para `optionalDependencies` declarando o motivo:

> *"melhorar compatibilidade com casos como registries customizados, proxies customizados,
> instalações offline, sistemas de arquivos somente-leitura, ou quando scripts de post-install estão
> desabilitados."*

É a descrição literal do ambiente corporativo que originou os nossos dois CLIs. **O mercado resolveu
o nosso problema, e nós reagimos a um wrapper quebrado escrevendo 53.744 linhas.**

Com `optionalDependencies` (npm) e wheels de plataforma (PyPI), o binário vem **do próprio
registry** — atravessa Artifactory e Nexus como qualquer outro pacote, sem tocar o GitHub.

#### Por que D supera C (WebAssembly)

| | C — WASM | D — binário embrulhado |
|---|---|---|
| reimplementação | zero | zero |
| dependência extra no Python | 🔴 `wasmtime` (que traz binário nativo) | nenhuma |
| problema de *preopen* de diretório | 🔴 sim | não |
| custo de tamanho e desempenho | sim | nenhum |
| precedente em produção | pouco | **esbuild, ruff, uv, biome** |

**A assimetria Node/Python que a opção C revelava desaparece em D.** O `wasmtime` era o que
condenava o canal pip; sem ele, os dois canais sobrevivem pelo mesmo mecanismo.

---

## Atualização de 2026-09-12 — a premissa original caiu

O KG informou que **a restrição de baixar executáveis vinha da empresa onde ele trabalha, e essa
empresa não vai mais usar o trackfw** — decidiu, com base nele, construir um CLI interno.

🔴 **O único caso concreto conhecido do público que justificou Node e Python deixou de existir.**

Isso, somado à medição acima, significa que hoje pagamos 53.744 linhas e 31 gates por um público de
que não temos **nenhuma** evidência, e cujo único exemplo conhecido saiu.

**Mas a opção D torna a pergunta quase irrelevante**: se manter os canais custa algumas centenas de
linhas de casquinha em vez de 53.744 de reimplementação, não é preciso apostar em qual público
existe. Mantém-se a opcionalidade **e** elimina-se a dívida.

## Decisão

**Nenhuma ainda — mas a direção mudou.** Registram-se quatro posições:

1. 🔴 **Não descontinuar nada com os dados atuais.** Seis stars e um usuário externo é pré-adoção;
   decidir distribuição aqui é decidir no ruído, e é irreversível na prática.
2. 🔴 **Não criar o instalador de Windows agora.** Ele não atende a restrição que originou os canais
   e serve um canal com zero demanda medida (0–2 downloads do tarball de Windows em toda a história).
3. 🔴 **A opção D é a candidata, não a C nem a descontinuação.** Ela remove as 53.744 linhas e os 31
   gates **preservando os três canais**, sem as ressalvas do WASM e com quatro precedentes em
   produção — um deles (esbuild) em Go, distribuído por npm.
4. **Parar de fazer a dívida crescer.** A regra dura de paridade continua valendo, mas feature nova
   deve ser avaliada contra a opção D antes de ser triplicada.

## Quando decidir

Esta ADR vira `Accepted` com decisão real quando existir **pelo menos um** destes:

- um usuário externo identificável instalando por npm ou pip — hoje: **zero**;
- evidência direta da restrição corporativa (issue, e-mail, conversa) — hoje: **zero**;
- adoção do tap acima de ~100 únicos/mês, indicando que o canal nativo basta — hoje: ~34/mês
  extrapolado;
- 🔴 **um protótipo da opção D**: publicar o binário Go num pacote npm de teste com
  `optionalDependencies` por plataforma e num wheel `py3-none-<plataforma>`, e medir se o
  comportamento é idêntico ao nativo nos três SOs.

**O quarto item é o único que depende só de nós, e é o de maior valor** — e mudou: não é mais o
protótipo de WASM, é o da opção D, que tem precedente em produção e nenhuma das ressalvas do WASM.
Custa um protótipo descartável e responde se dá para apagar 53.744 linhas sem perder canal nenhum.

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
- 🔴 **A opção D foi levantada DEPOIS das outras três, ao pesquisar prior art externo.** As três
  primeiras foram raciocinadas a partir do repositório. A lição é a mesma da §2.0 do `CLAUDE.md`
  (varrer issues antes de escrever roadmap), aplicada a **prior art de mercado**: verificar como o
  problema já foi resolvido fora, antes de projetar solução própria. O primeiro parecer do arquiteto
  recomendou WASM sem ter feito essa verificação.

---

## Decisões de adoção — 2026-09-12 (KG)

Duas questões que ficaram abertas na aceitação foram decididas. Registro com o ajuste que proponho
em cada, para a decisão ficar completa e não voltar como dúvida.

### 1. Os sítios de versão passando de 5 para 11+

**Decisão do KG:** *"podemos criar um gate e o problema está resolvido."*

**Concordo — com um ajuste que sai mais barato.** Um gate policiando 11 arquivos escritos à mão é
mais caro e mais frágil do que **gerar os 11 a partir de um**.

```
hoje              5 sítios escritos à mão, conferidos só no `release tag`  (issue #338)
gate sobre 11+    11 sítios à mão + gate vigiando        ← funciona, mas mantém a classe viva
GERAR + gate      1 sítio real; os manifests de plataforma são artefato de build
```

Os `package.json` dos pacotes de plataforma **não precisam existir no repositório** — o esbuild os
gera no release. Com geração, o número de sítios **cai de 5 para 1**, e o gate deixa de vigiar
divergência para verificar que a geração aconteceu.

🔴 **A diferença importa por um motivo que este projeto já pagou:** gate detecta *drift*; geração
**impede** o drift. A ADR de causa raiz deste repositório é explícita — registro não é correção.

**Consequência:** o **#338 deixa de piorar com a opção D e passa a ser resolvido por ela.** De
pré-requisito, vira entregável.

### 2. A medição de retorno (ML-2A da trilha 2)

**Decisão do KG:** *"nem precisamos saber disso; sabendo que diminuiremos pela metade os issues já
vale o risco."*

**Concordo quanto à decisão, e proponho realocar a medição em vez de cancelá-la.**

Por que concordo: o termo dominante da conta **já está medido**, e não é o número de issues —

```
53.744 linhas de reimplementação        medido
31 de 61 gates existindo só p/ paridade medido
```

Isso sozinho justifica a mudança. A contagem de issues é confirmação, não fundamento.

🔴 **Uma ressalva que preciso deixar escrita, porque a decisão passou a se apoiar nela:** o
*"8 de 16 issues desaparecem"* é **classificação preliminar do arquiteto, por leitura** — não
medição. Pode ser 5, pode ser 11. Não deve ser citado como fato medido em changelog, PR ou
comunicação externa.

**Por que a medição continua necessária, só que depois:** o valor dela nunca foi decidir; é saber,
**depois de adotar D**, quais das 33 REQs e dos 16 issues fecham por *causa removida* em vez de
ficarem abertos para sempre. Sem isso, adotamos a opção D e o backlog fica em limbo — ninguém sabe o
que ainda é trabalho.

**Realocada:** deixa de ser pré-requisito da decisão e passa a ser **entregável da execução da v8**,
junto com o fechamento em massa dos artefatos cuja causa desapareceu.

### O que ainda gate a adoção

Sobra **um** item, e é o modo de falha conhecido da opção D:

- **AC6** — `package-lock.json` gerado no macOS instalando no **Windows real**, com o binário de
  plataforma presente. É o bug clássico de `optionalDependencies` + lockfile.

Em execução na VM de Windows (`powershell-vm`) neste momento. 🔴 **É o único AC cuja falha ainda
reverteria a adoção.**
