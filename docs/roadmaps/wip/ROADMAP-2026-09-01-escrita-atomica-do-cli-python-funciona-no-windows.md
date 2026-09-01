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
**Status:** ⬜ Pendente
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
- [ ] Veredito sobre a janela de TOCTOU ser alcançável **neste** código, com evidência
- [ ] Enumeração de APIs POSIX-only além das três conhecidas
- [ ] Veredito sobre extrair vs. manter-e-gatear a triplicação
- [ ] Nenhuma linha de implementação escrita
- [ ] Parecer em `docs/seguranca/2026-09-01-modelo-de-ameaca-da-escrita-atomica-no-windows.md`

**Gates da wave:**
```bash
test -f docs/seguranca/2026-09-01-modelo-de-ameaca-da-escrita-atomica-no-windows.md
! grep -qi "placeholder" docs/seguranca/2026-09-01-modelo-de-ameaca-da-escrita-atomica-no-windows.md
grep -q "Residual" docs/seguranca/2026-09-01-modelo-de-ameaca-da-escrita-atomica-no-windows.md
```

## Wave 1 — A correção
> Dependências: Wave 0. **O particionamento sai do veredito sobre a triplicação** — extrair helper e
> manter-e-gatear produzem MLs diferentes. Escrever agora seria escopo inventado.

## Wave 2 — Gate e contrato
> Dependências: Wave 1. Gate falsificável + o contrato em `docs/cli-parity.md`.
> 🔴 Nasce ligado, com guarda de vacuidade ancorada no mesmo cwd da varredura, `python3` nunca
> `python`. Contrato escrito nesta sessão, depois de dois gates chegarem inertes.

## Verificação que só o CI fecha

A camada 2 **não mede este defeito** — nenhum dos 11 itens o cobre. A verificação em Windows real é a
**camada 1** (`windows-full-suites`), cuja linha de base é `145 failed, 1422 passed`. Se as três
escritas atômicas passarem a funcionar, a contagem de falhas **deve cair**. **Verificar quanto antes
de fixar número** — foi o passo que faltou na REQ do port, quando previ 3 e o CI deu 5.

## Barreira final

`hefesto-tf` e `hades-tf`, auditoria do arquiteto, `barrier`. **CI verde**, não só verde local.
