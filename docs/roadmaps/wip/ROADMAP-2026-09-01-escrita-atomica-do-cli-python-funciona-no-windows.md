---
status: wip
date: 2026-09-01
req: "docs/req/REQ-2026-09-01-os-fchmod-e-unix-only-e-derruba-as-tres-escritas-atomicas-do-cli-python-no-windows.md"
squad: "hades-tf, apolo-tf"
---

# Roadmap: Escrita atômica do CLI Python funciona no Windows

> Created: 2026-09-01 | Status: wip

## Context

REQ: `docs/req/REQ-2026-09-01-os-fchmod-e-unix-only-e-derruba-as-tres-escritas-atomicas-do-cli-python-no-windows.md`

`os.fchmod` é **Unix-only**. Três `_atomic_write` replicados o chamam, e em Windows levantam
`AttributeError` — **crash, não degradação**. Atinge `init --ai-tools`, `agents install`,
`skills install` e install de terceiro: **o onboarding inteiro, em toda máquina Windows**.

🔴 **A troca óbvia é a errada.** `os.chmod(path)` reintroduz **TOCTOU**; `os.fchmod(fd)` não tem
janela. Os arquivos protegidos são registro de quarentena, manifesto de integrações e identidade —
justamente os que não se pode enfraquecer. **Consertar o Windows quebrando o POSIX trocaria um crash
barulhento por falha silenciosa.**

## Acceptance Criteria

- [ ] As três escritas funcionam em Windows
- [ ] `os.fchmod` **continua sendo usado onde existe** — fallback condicional, nunca substituição
- [ ] Falsificação nas duas direções, **incluindo o controle** de que o caminho POSIX é exercido
- [ ] A duplicação tripla é tratada por decisão explícita, não replicada
- [ ] Gate falsificável, nascido ligado e com guarda de vacuidade
- [ ] Contrato escrito em `docs/cli-parity.md`
- [ ] `make quality` e **CI** verdes

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Modelo de ameaça da troca de primitiva
> Dependências: nenhuma. Bloqueia a implementação.

### ML-0A — TOCTOU, escopo real e a decisão sobre a triplicação
**Status:** ✅ Concluído
**Agente:** `hades-tf`
**Files affected:** nenhum (documento em `docs/seguranca/`)
**Por que esta Wave 0 é sobre segurança e não sobre portabilidade:** o defeito é trivial de
contornar; **o remédio é que tem risco**. Trocar `fchmod(fd)` por `chmod(path)` conserta o Windows e
**abre TOCTOU no POSIX**, nos três arquivos mais sensíveis do projeto.
**Actions:**
1. **A janela é real ou teórica neste código?** Os três usam `tempfile.mkstemp` num diretório criado
   com `0o700`. Quem consegue vencer a corrida, com que capacidade? Se o diretório pai já é
   restritivo, a janela pode ser inalcançável — **e nesse caso a resposta muda**. Meça, não presuma.
2. **Enumeração:** só estes três? Varra o `pypi/` por outras chamadas de API POSIX-only
   (`os.fchmod`, `os.fchown`, `os.chown`, `os.geteuid`, `os.getuid`, `os.symlink` sem guarda,
   `os.link`, `stat.S_IS*` usados como decisão de segurança). **Assuma que minha lista de três está
   incompleta** — nesta sessão duas enumerações minhas erraram por uma ordem de grandeza, e nas duas
   você achou a população real.
3. **Falsificação nas duas direções.** Direta: o fallback não dispara em POSIX. **Simétrica, e é a
   que me preocupa:** um fallback que dispare em POSIX por engano (ex.: `getattr` mal escrito,
   `os.fchmod` sombreado em teste) **silenciosamente enfraquece a garantia sem falhar nenhum teste**.
4. **A decisão sobre a triplicação.** O `quarantine.py:34-37` declara a replicação **deliberada**,
   para manter o pacote `thirdparty` independente de `trackfw.integrations`. Extrair um helper
   **contraria essa decisão registrada**. Julgue: a independência ainda vale mais que o risco de
   corrigir dois de três? Se sim, o remédio é **gate que garanta não divergirem**, não extração.
5. **Residual declarado.**
**Critérios de aceite:**
- [x] Veredito sobre a janela de TOCTOU ser alcançável **neste** código, com evidência
- [x] Enumeração de APIs POSIX-only além das três conhecidas
- [x] Veredito sobre extrair vs. manter-e-gatear a triplicação
- [x] Nenhuma linha de implementação escrita
- [x] Parecer em `docs/seguranca/2026-09-01-modelo-de-ameaca-da-escrita-atomica-no-windows.md`

**Gates da wave:**
```bash
test -f docs/seguranca/2026-09-01-modelo-de-ameaca-da-escrita-atomica-no-windows.md
! grep -qi "placeholder" docs/seguranca/2026-09-01-modelo-de-ameaca-da-escrita-atomica-no-windows.md
grep -q "Residual" docs/seguranca/2026-09-01-modelo-de-ameaca-da-escrita-atomica-no-windows.md
```

#### Resultado do ML-0A (hades-tf, 2026-09-01) — auditado pelo arquiteto

**A Wave 0 derrubou dois critérios meus, os dois pela mesma razão: eu tinha escrito asserções que
passariam sem provar nada.**

### 1. A AC3 que escrevi era vácua — em 5 dos 7 sites

Eu pedia *"provar que `os.fchmod` é chamado"*. **Medi e confirmei:** `tempfile.mkstemp()` já entrega
`0o600` por padrão, e **5 dos 7 pontos de chamada pedem exatamente `0o600`**. Ali o `fchmod` **não
tem efeito observável** — um teste que instrumenta a chamada fica verde sem provar garantia alguma.

Os únicos dois sites com efeito real são `integrations/manager.py:343` e `:358`, que recebem `mode`
da linha `585` (`0o644`). **O controle tem de mirar lá e verificar o resultado** —
`st_mode & 0o777 == 0o644` — **não a instrumentação.**

**Escrevi um critério vácuo dentro da REQ cujo propósito é impedir vacuidade.** AC3 corrigida.

### 2. A AC6 teria publicado um contrato FALSO

Eu ia escrever em `docs/cli-parity.md` que *"os 3 runtimes preservam a garantia de descritor"*. O
ML-0A mediu que **o Node já tem a mesma classe de TOCTOU hoje, em produção, sem relação com
Windows**: `chmodSync(path)` em vez de `fchmodSync(fd)` — **que existe no Node** — e o `manager.js`
ainda chama `chmod` **uma segunda vez depois do `rename`**, janela extra que o `identity.js` do
próprio Node não tem.

**Contrato falso é pior que contrato ausente: compra confiança que não existe.** AC6 corrigida para
nomear a exceção; **REQ própria aberta** para o Node.

### 3. A janela é real — provada por execução, não por argumento

PoCs no parecer demonstram corrupção de permissão de arquivo alheio via troca de symlink, e o
alargamento `0o600 → 0o644` com o literal que o código de fato usa.

**E o pré-requisito que eu supunha protetor não é garantido pelo nosso próprio código:** o `mode=` de
`os.makedirs`/`Path.mkdir` é **ignorado quando o diretório já existe** e **não se propaga a pais
intermediários** criados por `parents=True`. Ele reproduziu os dois cenários ao vivo — `.trackfw`
frequentemente fica `0o755`, não `0o700`. Sob umask padrão não é explorável por outro usuário; passa
a ser sob umask relaxado. **Nota de vault escrita.**

### 4. Enumeração — desta vez sem subestimativa

`os.fchmod` é a **única** API POSIX-only usada como decisão de segurança em `pypi/`. Varredura
ampliada incluiu `tests/`, `st_ino`/`st_dev`, `O_NOFOLLOW`, `os.lchmod`, `shutil.chown` — zero
ocorrências. Os três da REQ batem.

### 5. Triplicação: **não extrair** — e por um motivo que eu não sabia

`references.py` e `provenance.py` **já reusam** o `quarantine._atomic_write` **por import**, não
duplicam. A triplicação real é só `identity/__init__.py` + `integrations/manager.py` +
`quarantine.py`. Remédio: **gate estrutural anti-divergência**, mais doc-comment em
`identity/__init__.py` — a única das três **sem justificativa registrada**.

## Wave 1 — A correção (2 MLs)
> Dependências: Wave 0. Particionamento derivado do veredito: **não extrair**, logo a correção é
> pontual nos três e o gate garante a não-divergência.

### ML-1A — Fallback condicional nos três `_atomic_write`
**Status:** ✅ Concluído
**Agente:** `apolo-tf`
**Files affected:** `pypi/trackfw/identity/__init__.py`, `pypi/trackfw/integrations/manager.py`,
`pypi/trackfw/thirdparty/quarantine.py`
**Actions:**
1. Fallback **condicional** — `getattr(os, "fchmod", None)` —, nunca substituição incondicional. Em
   POSIX o comportamento fica **byte a byte o de hoje**.
2. Doc-comment em `identity/__init__.py` registrando a replicação deliberada, como
   `quarantine.py:34-37` já faz. É a única das três sem justificativa escrita.
**Critérios de aceite:**
- [x] As três concluem sem `AttributeError` quando `os.fchmod` está ausente
- [x] 🔴 **Controle no site de `0o644`** (`manager.py:343`/`:358`), verificando
      `st_mode & 0o777 == 0o644` — **o resultado, não a chamada**. Nos sites de `0o600` a asserção
      seria vácua, porque `mkstemp` já entrega esse modo.
- [x] 🔴 O fallback **não dispara em POSIX** — provar, porque um fallback disparando por engano
      enfraquece a garantia **sem falhar nenhum teste**
- [x] `make quality` verde

#### Resultado do ML-1A (apolo-tf, 2026-09-01) — auditado pelo arquiteto

**O achado dele teria anulado o ML inteiro, e é o mesmo padrão que a Wave 0 me impôs.**

Ele ia colocar `pytestmark = skipif(os.name != "posix", ...)` no nível do módulo. **Isso pularia os
7 testes no runner `windows-full-suites`** — precisamente a camada que este roadmap nomeia como a
verificação real. **O teste que prova o comportamento no Windows seria pulado no Windows.**

Escopou o skip para **apenas os 3 testes de espionagem**, e condicionado a
`hasattr(os, "fchmod")` — **condição medida, não nome de plataforma**. Verifiquei: 7 testes, 3 com
skip. É a mesma disciplina do `symlinkOrSkip` do ML-2A da REQ do instrumento: **pular por condição
medida, nunca por palpite de plataforma.**

### O controle mira o site certo

```python
IntegrationManager._atomic_write(target, b"payload", 0o644)   # o único site com efeito observável
```

Verifica o **`st_mode` resultante, não a chamada** — a correção que o ML-0A impôs à minha AC vácua.
Rodei os 7 aqui: **passam**.

E ele antecipou um detalhe que eu não tinha: **no Windows, `0o644` pode ser lido de volta como
`0o666`**, então a asserção de modo fica restrita a POSIX enquanto a de *"não lança
`AttributeError`"* roda em todo lugar. **Asserção certa em cada plataforma**, em vez de uma só,
frouxa nas duas.

### Falsificação executada, não deduzida

| direção | sabotagem | resultado |
|---|---|---|
| (a) | fallback removido, `os.fchmod` nu | `AttributeError` em `manager.py:120`; 2 dos 7 vermelhos |
| (b) | fallback **incondicional** | `test_..._uses_fchmod_not_chmod_on_posix` vermelho — o espião pegou o fallback disparando com `os.fchmod` presente |

A direção (b) é a que me preocupava: um fallback disparando por engano em POSIX **enfraquece a
garantia sem falhar nenhum teste** — a menos que exista esse controle. Agora existe.

### Um falso positivo que ele diagnosticou corretamente

A primeira rodada deu `FAIL [falsify/no-repo-mutation]` porque ele editava o
`docs/agents-working-context.md` **enquanto** a janela de `git status` do gate estava aberta.
**Não é defeito** — é o gate funcionando. Ele identificou, refez limpo: `MAKE_EXIT=0`,
`OK [falsify/no-repo-mutation]`, suíte `pypi` com 1582 passed.

### ML-1B — Gate anti-divergência e contrato com a exceção do Node
**Status:** ⬜ Pendente
**Agente:** `artemis-tf`
**Files affected:** `scripts/` (gate novo), `Makefile`, `docs/cli-parity.md`
**Actions:**
1. Gate estrutural garantindo que as **três** cópias de `_atomic_write` não divirjam. **Corrigir duas
   de três é o modo de falha mais provável** — o veredito de não extrair depende deste gate existir.
2. Contrato em `docs/cli-parity.md` **com a exceção do Node nomeada** e apontando para a
   `REQ-2026-09-01-cli-node-usa-chmodsync-...`. **Não escrever "os 3 runtimes preservam a garantia"** —
   seria falso hoje.
**Critérios de aceite:**
- [ ] Gate falsificado nas duas direções: com as três iguais passa; divergindo **uma**, reprova
- [ ] 🔴 Nasce **ligado** ao `Makefile`, com **guarda de vacuidade ancorada no mesmo cwd** da
      varredura, e `python3` nunca `python`
- [ ] O contrato **não afirma** o que o Node não cumpre
- [ ] `make quality` verde

## Verificação que só o CI fecha

A camada 2 **não mede este defeito** — nenhum dos 11 itens o cobre. A verificação em Windows real é a
**camada 1** (`windows-full-suites`), cuja linha de base é `145 failed, 1422 passed`. Se as três
escritas atômicas passarem a funcionar, a contagem de falhas **deve cair**. **Verificar quanto antes
de fixar número** — foi o passo que faltou na REQ do port, quando previ 3 e o CI deu 5.

## Barreira final

`hefesto-tf` e `hades-tf`, auditoria do arquiteto, `barrier`. **CI verde**, não só verde local.
