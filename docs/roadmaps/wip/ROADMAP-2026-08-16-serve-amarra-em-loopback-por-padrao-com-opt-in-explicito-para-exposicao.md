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
- [x] AC1 — Padrão nos 3 CLIs é **loopback**, verificado por `lsof`/equivalente (não por leitura de código).
- [x] AC2 — Exposição é **opt-in explícito** (`--host`), nunca padrão.
- [x] AC3 — Ao expor, **aviso claro** de leitura sem autenticação.
- [x] AC3b — A **URL impressa** (e a aberta no browser) reflete o `--host` efetivo, não `localhost` fixo.
- [x] AC4 — Aviso **renderizado** byte-idêntico nos 3 + string de help **da flag** byte-idêntica no fonte.
- [ ] AC5 — Gate de paridade do endereço padrão + cenário de falsificação (P4), usando `--host ::1` como discriminante. (ML-1C, ainda pendente)
- [x] AC6 — Não-regressão: dashboard continua acessível em `localhost` como hoje.
- [x] AC7 — `make quality` verde ao fim do ML-1B (log completo colado no relatório do agente). ML-1C
  vai tocar `scripts/check-gates-falsify.sh` — **revalidar `make quality` ao fim do ML-1C**, não
  reusar esta evidência como definitiva.
- [x] AC8 — `--host ::1` funciona nos **3** CLIs (hoje só no Node).

> **AC4 foi reescrito.** "`--help` byte-idêntico nos 3" é inverificável: cobra imprime `Flags:`,
> commander `Options:`, argparse `options:` + linha `usage:`. As 2 ocorrências de `--host` no help do
> Python eram a synopsis do argparse, **não** duplicação de texto.

## Evidência de auditoria (medida pelo arquiteto em 2026-08-16, não lida do código)

```
lsof:  tfw IPv4 127.0.0.1:45901 · node IPv4 127.0.0.1:45902 · Python IPv4 127.0.0.1:45903
localhost -> 200/200/200   127.0.0.1 -> 200/200/200   [::1] -> 000/000/000   LAN -> 000/000/000
```
Aviso byte-idêntico nos 3 com `--host 0.0.0.0` e com o IP da LAN. AC1, AC2, AC3, AC4 e AC6 fecham.

---

## Wave 1 — Correção

### ML-1A — Loopback por padrão + `--host` opt-in + aviso, nos 3 CLIs
**Status:** ✅ Concluído — auditado pelo arquiteto por medição (`lsof`/`curl`), não por leitura
· **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
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
- [x] Com o padrão, `lsof` mostra escuta em **loopback** nos 3 CLIs — evidência colada no relatório.
- [x] Acesso pelo **IP da LAN** com o padrão: **recusado**.
- [x] Com `--host 0.0.0.0`: acessível **e** com o aviso impresso.
- [x] `http://localhost:<porta>/` continua **200** com o padrão, nos 3.
- [x] Aviso byte-idêntico nos 3 (por `diff` das saídas reais) + help **da flag** idêntico no fonte —
  ver o AC4 reescrito acima: `--help` integral byte-idêntico é inverificável entre cobra/commander/argparse.
- [x] `make quality` verde (revalidado após o rebase na `main`).

**Comando de validação:** `make quality`

---

## Wave 1b — Fechamento do ML-1A (depende da auditoria acima)

### ML-1B — `--host ::1` nos 3 CLIs + URL impressa reflete o host efetivo
**Status:** ✅ Concluído — auditado por medição (`lsof`/`curl`), evidência abaixo e em
`docs/agents-working-context.md` · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Arquivos:** `internal/serve/serve.go`, `internal/commands/serve.go`,
`pypi/trackfw/commands/serve.py`, `npm/src/commands/serve.js` + testes dos 3.

**Diagnóstico medido — não é hipótese:**

| CLI | `--host ::1` | causa raiz |
|---|---|---|
| Go | ❌ `too many colons in address` | `fmt.Sprintf("%s:%d", host, port)` não põe colchetes |
| Node | ✅ escuta em `[::1]` | referência, **não alterar o bind** |
| Python | ❌ `nodename nor servname provided` | `HTTPServer` é `AF_INET` fixo |

**Ações:**
1. **Go:** trocar `fmt.Sprintf("%s:%d", host, port)` por `net.JoinHostPort(host, strconv.Itoa(port))`.
2. **Python:** subclasse de `HTTPServer` com `address_family = socket.AF_INET6` quando o host for
   IPv6 (`ipaddress.ip_address(host).version == 6`), `AF_INET` caso contrário.
3. **URL impressa (AC3b):** os 3 imprimem `http://localhost:<porta>` seja qual for o `--host`. Com
   `--host 192.168.x.y` a URL não é alcançável e Node/Python **abrem o browser no endereço errado**.
   Imprimir o host efetivo, com colchetes quando IPv6 (`http://[::1]:4080`). Manter `localhost`
   apenas quando o host for loopback IPv4, para não mudar a saída do caso comum.
4. **Go:** hoje a linha "listening on ..." é impressa **duas vezes** (`internal/commands/serve.go` e
   `internal/serve/serve.go`) e **antes** de o bind falhar — a saída afirma que está escutando quando
   não está. Consolidar em uma linha, emitida só após o listener subir (`net.Listen` + `http.Serve`).

**🔴 Onde este ML pode falhar em silêncio:**
- **Regressão no caso comum.** `trackfw serve` sem flags é o uso de 99% — a saída e o
  comportamento em `localhost` **não podem mudar**. Meça com `lsof` + `curl`, não por leitura.
- **Não mexer no bind do Node.** Ele já é a referência correta; alinhar Go e Python a ele.
- **Aviso deve continuar byte-idêntico** nos 3 depois da mudança (AC4 já fecha hoje — não quebre).

**Critérios de aceite:**
- [ ] `--host ::1` → `lsof` mostra `[::1]` e `curl http://[::1]:<porta>/` devolve **200** nos 3.
- [ ] Padrão sem flags → `lsof` mostra `127.0.0.1` e `curl localhost` devolve **200** nos 3 (não-regressão).
- [ ] `--host 192.168.x.y` → URL impressa contém o IP, não `localhost`, nos 3.
- [ ] Go imprime a linha de listening **uma vez** e **não** a imprime quando o bind falha.
- [ ] Aviso de exposição segue byte-idêntico nos 3 (`diff` das saídas reais, não do fonte).
- [ ] Evidência de `lsof`/`curl` colada no relatório para **cada** CLI.

### ML-1C — Gate de paridade do endereço padrão + cenário P4
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Arquivos:** `scripts/check-gates-falsify.sh` + script de gate de paridade.
**Dependência:** ML-1B completo (o discriminante depende do comportamento final).
**Ação:** gate que verifica o endereço de escuta **padrão** nos 3 por execução real, e cenário P4
com braço baseline + braço de detecção, seguindo a estrutura já existente no script (ver os cenários
`roadmap-move-parity` e `barrier/*` como modelo — `assert_fails_with` / `assert_lacks_pattern` e a
prova de não-vacuidade com a regra desligada). **Discriminante:** `--host ::1`.

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
- **Não-objetivo declarado — loopback dual-stack.** O padrão é IPv4 `127.0.0.1` nos 3. Um único
  listener não cobre `127.0.0.1` e `::1` ao mesmo tempo, e o wildcard `:porta` é exatamente o bug
  corrigido aqui. Medido: `curl localhost` devolve **200** nos 3 com bind IPv4 (o cliente faz
  fallback). Dois listeners só entrariam se `localhost` falhasse em algum runtime — e não falha.
- **Fora de escopo, declarado:** `--host ::ffff:127.0.0.1` (IPv4-mapped) e `--host 127.0.0.2`. Nos
  dois casos **nenhum dos 3 CLIs consegue escutar**, logo o impacto de segurança é nulo; o que
  diverge é só a mensagem de erro. Registrado, não corrigido.
- **Observação, não escopo:** o default de porta diverge (Go 4080 · Node/Python 8080). Pré-existente
  a esta REQ.
- **Fora de escopo, declarado:** autenticação no `serve`. Se a exposição vier a ser intencional, vira
  REQ própria — misturar atrasaria a correção do padrão inseguro, que é o urgente.
- Commits e branch são exclusivos do `trackfw_architect`.
