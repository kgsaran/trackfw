---
status: Done
date: 2026-09-01
author: "zeus-tf"
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-09-01-escrita-atomica-do-cli-python-funciona-no-windows.md"
---

# REQ: `os.fchmod` é Unix-only e derruba as três escritas atômicas do CLI Python no Windows

> Date: 2026-09-01 | Status: Open

## Motivation

Achado **13** de `lourivalgarciajunior` no issue #216, triado em
`docs/analises/2026-09-01-triagem-dos-novos-achados-do-issue-216.md` e **confirmado por leitura**.

`os.fchmod` é **Unix-only** (`Availability: Unix` na documentação do CPython). O CLI Python o chama
em **três** implementações replicadas de `_atomic_write`:

| arquivo | linha |
|---|---|
| `pypi/trackfw/identity/__init__.py` | 97 |
| `pypi/trackfw/thirdparty/quarantine.py` | 42 |
| `pypi/trackfw/integrations/manager.py` | 120 |

Em Windows, as três levantam `AttributeError` — **não é degradação, é crash**.

### O que isso derruba, e por que supera a fila

Atinge `init --ai-tools`, `agents install`, `skills install` e o install de artefato de terceiro:
**o caminho de onboarding, em toda máquina Windows, sempre.** Os itens que estavam à frente na fila
são latentes em comparação — o item 10 exigia commit no Windows *e* checkout no Linux para se
manifestar.

### Os outros dois runtimes não têm o defeito, e o Go mostra o remédio certo

```
Go      temporary.Chmod(mode)        descritor — funciona no Windows
Node    fs.chmodSync(tmp, mode)      caminho   — funciona no Windows
Python  os.fchmod(descriptor, mode)  descritor — Unix-only, AttributeError
```

### 🔴 A troca incondicional que o autor ofereceu enfraqueceria o POSIX

O caminho óbvio — trocar `os.fchmod(descriptor, mode)` por `os.chmod(path, mode)` — **não é o
remédio**, e a razão é de segurança, não de estilo:

- `os.fchmod(fd)` opera no **descritor já aberto**. Entre o `mkstemp` e o `chmod` não há janela: o
  descritor **é** o arquivo.
- `os.chmod(path)` opera no **caminho**, e reintroduz **TOCTOU** — o caminho pode ser substituído
  entre a criação e a permissão.

Esses arquivos são exatamente os que não se pode enfraquecer: registro de quarentena de terceiro,
manifesto de integrações e identidade. **Consertar o Windows quebrando a garantia do POSIX seria
trocar um crash barulhento por uma falha silenciosa** — e falha silenciosa é a classe que nos custou
sete ocorrências nesta sessão.

## Acceptance Criteria

- [x] **AC1** — As três escritas atômicas funcionam em Windows.
- [x] **AC2** — 🔴 **`os.fchmod` continua sendo usado onde existe.** O fallback é **condicional**
      (`getattr(os, "fchmod", None)`), nunca substituição incondicional. Em POSIX o comportamento é
      **byte a byte o de hoje**.
- [x] **AC3** — 🔴 **Falsificação nas duas direções.** (a) simulando a ausência de `os.fchmod`, as
      três escritas concluem sem `AttributeError`; (b) **controle no site certo.**

      ⚠️ **A primeira redação desta AC era vácua e foi corrigida pelo ML-0A.** Ela pedia *"provar que
      `os.fchmod` é chamado"* — e isso **passaria vacuamente em 5 dos 7 pontos de chamada**, porque
      `tempfile.mkstemp()` **já entrega `0o600` por padrão** (medido: `oct(0o600)`), exatamente o
      modo que 5 dos 7 sites pedem. Ali o `fchmod` não tem efeito observável, e um teste que
      instrumenta a chamada ficaria verde sem provar garantia nenhuma.

      O controle tem de mirar os **dois** sites onde o modo difere do padrão do `mkstemp` —
      `integrations/manager.py:343` e `:358`, que recebem `mode` da linha `585` (`0o644`) — e
      verificar o **resultado observável**: `st_mode & 0o777 == 0o644`. **Testar o efeito, não a
      instrumentação.**
- [x] **AC4** — **A duplicação é tratada, não replicada.** São três cópias de `_atomic_write`, com
      doc-comment declarando a replicação deliberada (`quarantine.py:34-37`: manter o pacote
      `thirdparty` independente de `trackfw.integrations`). **Decidir explicitamente**: extrair para
      um helper compartilhado, ou manter as três e **garantir por gate** que não divirjam. Corrigir
      duas de três é o modo de falha mais provável.
- [x] **AC5** — Gate falsificável cobrindo a AC2 e a AC4. **Nasce ligado ao `Makefile`, com guarda de
      vacuidade ancorada no mesmo cwd da varredura, e `python3` nunca `python`** — contrato em
      `docs/cli-parity.md`.
- [x] **AC6** — O contrato sobre escrita atômica é escrito em `docs/cli-parity.md`.

      ⚠️ **A redação que eu propus seria FALSA se publicada.** O ML-0A mediu que **o CLI Node já tem
      a mesma classe de TOCTOU hoje, em produção, sem relação alguma com Windows**:
      `npm/src/thirdparty/quarantine.js:28-30` e `npm/src/integrations/manager.js:94-97` usam
      `chmodSync(path, mode)` em vez de `fchmodSync(fd, mode)` — **que existe no Node**. E o
      `manager.js` chama `chmod` **uma segunda vez depois do rename**, uma janela extra que o próprio
      `identity.js` do Node não tem.

      Escrever *"os 3 runtimes preservam a garantia de descritor"* seria **afirmar um contrato que o
      Node não cumpre** — e um contrato falso é pior que contrato ausente, porque compra confiança.
      O contrato tem de ser escrito **com a exceção do Node nomeada**, e a correção do Node vira
      **REQ própria**, não expansão desta.
- [x] **AC7** — `make quality` verde e **CI verde**. A camada 2 **não** mede este defeito; a
      verificação em Windows real é a suíte completa (camada 1).

## Negative Scope

- **Não** unificar `_atomic_write` com Go/Node — são runtimes diferentes com primitivas diferentes.
- **Não** mexer no bit de execução em NTFS (item 3, já corrigido no PR #229).
- **Não** alterar as permissões pedidas (`0o600`/`0o700`), só o mecanismo de aplicá-las.

## REQ fechada em 2026-09-01

**Camada 1: `145 failed / 1422 passed` → `103 failed / 1477 passed`** (−42 falhas). A camada 2 ficou
em 4, **corretamente** — nenhum dos 11 itens da issue mede este defeito.

Ambas as barreiras aprovaram sem bloqueante. O ponto que decidia tudo foi falsificado ao vivo pelo
`hades-tf`: forçando o `else:` disparar com `os.fchmod` presente, o controle **pegou**. Em POSIX o
comportamento é **byte a byte o de antes**.

**Duas ACs minhas foram derrubadas por serem vácuas ou falsas**, ambas antes de virarem código:
a AC3 pedia *"provar que `fchmod` é chamado"* — vácua em 5 dos 7 sites; a AC6 ia publicar que *"os 3
runtimes preservam a garantia"* — falso, o Node não preserva.

**REQ de acompanhamento aberta** para a ressalva 1: o gate anti-divergência prova que as três cópias
concordam, **não que existam só três** — uma quarta passaria silenciosa para sempre.

## Linked ADR

ADR: <!-- avaliar na Wave 0: se a decisão for extrair helper compartilhado, isso contraria o
doc-comment que declara a replicação deliberada, e a mudança de postura precisa de registro. -->

## Linked Roadmap

Roadmap: `docs/roadmaps/done/ROADMAP-2026-09-01-escrita-atomica-do-cli-python-funciona-no-windows.md`
