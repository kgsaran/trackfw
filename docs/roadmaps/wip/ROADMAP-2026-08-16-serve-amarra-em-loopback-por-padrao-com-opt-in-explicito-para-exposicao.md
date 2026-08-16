---
status: wip
date: 2026-08-16
req: "docs/req/REQ-2026-08-16-trackfw-serve-escuta-em-todas-as-interfaces-sem-autenticacao-expondo-a-cadeia-de-governanca-na-rede.md"
squad: "apolo-tf, hades-tf"
---

# Roadmap: `serve` amarra em loopback por padrão, com opt-in explícito para exposição

> Created: 2026-08-16 | Status: wip | **Prioridade: urgente (KG)**

## Context

REQ: `docs/req/REQ-2026-08-16-trackfw-serve-escuta-em-todas-as-interfaces-sem-autenticacao-expondo-a-cadeia-de-governanca-na-rede.md`
Parecer de origem: `docs/seguranca/2026-08-16-vazamento-de-stack-no-cli-node.md`

`trackfw serve` no **Go** e no **Python** escuta em **todas as interfaces**, sem autenticação —
`/api/chain` devolve 105 KB com ADRs, REQs e roadmaps para qualquer dispositivo da rede. O **Node**
amarra em `127.0.0.1`, e é essa divergência que denuncia o descuido.

Medido pelo arquiteto: `TCP *:45871 (LISTEN)` e `HTTP 200` pelo IP da LAN.
Delimitado: `/api/file` recusa com **403**, inclusive path traversal — a restrição de caminho
funciona; o problema é a interface de escuta somada à ausência de autenticação.

| CLI | onde |
|---|---|
| Go | `internal/serve/serve.go:59` — `addr := fmt.Sprintf(":%d", port)`, `:61` `ListenAndServe` |
| Python | `pypi/trackfw/commands/serve.py:150` — `HTTPServer(("", port), handler_class)` |
| Node | `npm/src/commands/serve.js:151` — `server.listen(port, '127.0.0.1', ...)` ← referência |

## Acceptance Criteria
- [ ] AC1 — Padrão nos 3 CLIs é **loopback**, verificado por `lsof`/equivalente (não por leitura de código).
- [ ] AC2 — Exposição é **opt-in explícito** (`--host`), nunca padrão.
- [ ] AC3 — Ao expor, **aviso claro** de leitura sem autenticação.
- [ ] AC4 — Idêntico nos 3 CLIs, incluindo texto do aviso e do `--help`.
- [ ] AC5 — Gate de paridade do endereço padrão + cenário de falsificação (P4).
- [ ] AC6 — Não-regressão: dashboard continua acessível em `localhost` como hoje.
- [ ] AC7 — `make quality` verde.

---

## Wave 1 — Correção

### ML-1A — Loopback por padrão + `--host` opt-in + aviso, nos 3 CLIs
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Arquivos:** `internal/serve/serve.go`, `internal/commands/serve.go`,
`pypi/trackfw/commands/serve.py`, `npm/src/commands/serve.js` (alinhar flag/aviso, o bind já está
correto), script de paridade novo ou existente, `scripts/check-gates-falsify.sh`, + testes.

**Ações:**
1. **Go:** `addr` passa a usar o host resolvido, com padrão `127.0.0.1`.
2. **Python:** `HTTPServer((host, port), ...)` com padrão `127.0.0.1`.
3. **Node:** já amarra em loopback — acrescentar a flag e o aviso para ficar idêntico aos outros.
4. **Flag `--host`** nos 3, com o mesmo nome, o mesmo default e o mesmo texto de `--help`.
5. **Aviso ao expor:** quando `--host` resolver para algo diferente de loopback, imprimir aviso
   nomeando que **a cadeia de governança ficará legível sem autenticação** por qualquer dispositivo
   alcançável. Texto **byte-idêntico** nos 3.
6. **Gate de paridade** verificando o endereço de escuta padrão — e **P4**: cenário em
   `check-gates-falsify.sh` com braço baseline e braço detecção.

**🔴 Onde este ML pode falhar em silêncio:**
- **Testar por leitura de código em vez de por escuta real.** Um teste que só verifica a string
  `"127.0.0.1"` no código passaria mesmo se o bind efetivo mudasse. **O critério é o endereço em que
  o processo realmente escuta** — verificar com `lsof`, `ss`, `netstat` ou tentativa de conexão.
- **Quebrar o dashboard.** `serve` é usado o tempo todo; se `localhost` deixar de funcionar, a
  correção vira regressão. AC6 é obrigatório.
- **IPv6.** O Go hoje escuta em `*` e apareceu como IPv6 no `lsof`. Confirme que o padrão novo cobre
  o caso de forma consistente entre os 3 CLIs — `localhost` pode resolver para `::1` ou `127.0.0.1`.

**Critérios de aceite:**
- [ ] Com o padrão, `lsof` mostra escuta em **loopback** nos 3 CLIs — evidência colada no relatório.
- [ ] Acesso pelo **IP da LAN** com o padrão: **recusado**.
- [ ] Com `--host 0.0.0.0`: acessível **e** com o aviso impresso.
- [ ] `http://localhost:<porta>/` continua **200** com o padrão, nos 3.
- [ ] Aviso e `--help` byte-idênticos nos 3.
- [ ] `make quality` verde.

**Comando de validação:** `make quality`

---

## Wave 2 — Barreira

### ML-2A — `hades-tf`: confirmar que a exposição fechou
**Status:** ⬜ Pendente · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** seção apensada a `docs/seguranca/2026-08-16-vazamento-de-stack-no-cli-node.md`
**Ações:** confirmar **por conexão real** que o padrão não é alcançável fora da máquina nos 3 CLIs;
avaliar se o `--host` cria caminho de exposição acidental (ex.: alguém colocar em script ou config
versionada); e verificar se **outro** componente do produto abre porta (o `serve` era o suspeito
óbvio — procurar os não óbvios). **Veredito explícito; bloquear é saída legítima.**

---

## Notas
- **Fora de escopo, declarado:** autenticação no `serve`. Se a exposição vier a ser intencional, vira
  REQ própria — misturar atrasaria a correção do padrão inseguro, que é o urgente.
- Commits e branch são exclusivos do `trackfw_architect`.
