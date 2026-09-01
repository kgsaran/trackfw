---
status: Open
date: 2026-09-01
author: "zeus-tf"
adr: ""
roadmap: ""
---

# REQ: `serve` interpola `--host` em string de shell e permite injeção de comando ao abrir o browser

> Date: 2026-09-01 | Status: Open

## Motivation

Achado do `hades-tf` na Wave 0 da `REQ-2026-09-01-mesmo-gate-de-wave-...`, **fora do escopo daquela
REQ** — apareceu porque pedi a enumeração de *todo* ponto onde conteúdo vira processo, e não só os do
`barrier`.

`npm/src/commands/serve.js:46-56` — `displayUrl()` devolve `http://${host}:${port}` **sem
sanitização** para qualquer string que não seja `localhost` nem IP válido. O resultado é interpolado
numa **string de shell** e passado a `exec()`:

```javascript
// serve.js:208-211
if (platform === 'darwin') openCmd = `open "${url}"`
else if (platform === 'win32') openCmd = `start "" "${url}"`
else openCmd = `xdg-open "${url}"`
exec(openCmd, ...)
```

**Reproduzido:**

```
--host  'x" ; id > /tmp/INJETADO ; echo "'
   ↓
exec:   open "http://x" ; id > /tmp/INJETADO ; echo ":4080"
                       ↑ a aspa fecha        ↑ comando arbitrário
```

`pypi/trackfw/commands/serve.py:196` tem a variante Windows do mesmo problema:
`subprocess.Popen(["start", url], shell=True)` — com `shell=True`, a lista é reunida numa string.
Os ramos Darwin e Linux do Python usam `Popen(["open", url])` **sem shell** e estão corretos.

## Por que importa apesar de ser "o usuário se auto-injetando"

A superfície não é o terminal do usuário. É:

- **Script, `Makefile` ou alias** que monta `--host` a partir de variável de ambiente ou config.
- **Documentação copiada e colada** — o vetor clássico de comando com payload embutido.
- **Task de editor / harness de agente** que passa `--host` derivado de um arquivo do projeto.

E o modo de falha é silencioso: o browser abre, o `serve` sobe, e **o comando extra executa sem
nenhum sinal**. Diferente de um crash, aqui **o sucesso aparente é parte do defeito**.

## Acceptance Criteria

- [ ] **AC1** — Nenhum caminho interpola valor controlável em string de shell. Use **argv** —
      `spawn('open', [url])` no Node — em vez de montar comando. Os ramos Darwin e Linux do Python
      **já fazem isso** e são o precedente interno.
- [ ] **AC2** — 🔴 O ramo Windows do Python (`Popen(["start", url], shell=True)`) deixa de usar
      `shell=True`. `start` é builtin do `cmd.exe`, então a forma correta exige cuidado —
      `["cmd", "/c", "start", "", url]` com argv, **não** string.
- [ ] **AC3** — 🔴 **Falsificação nas duas direções.** (a) o payload acima **não** executa comando
      extra; (b) **controle:** `--host` legítimo — hostname com hífen, IPv6, domínio interno —
      **continua abrindo o browser**. Sem (b), trocaríamos injeção por `serve` que não abre nada.
- [ ] **AC4** — **Validar o `--host` na entrada**, não só escapar na saída. Um host que não é
      hostname, IPv4 nem IPv6 válido deve ser **recusado com mensagem clara** — defesa em
      profundidade, e o `displayUrl` deixa de ser o único ponto de contenção.
- [ ] **AC5** — Gate falsificável cobrindo AC1 e AC2 nos 3 CLIs. **Nasce ligado ao `Makefile`, com
      guarda de vacuidade ancorada no mesmo cwd, `python3` nunca `python`.**
- [ ] **AC6** — Paridade: os 3 CLIs abrem o browser pela mesma forma, e o contrato entra em
      `docs/cli-parity.md`.
- [ ] **AC7** — `make quality` e **CI** verdes.

## Negative Scope

- **Não** remover a abertura automática do browser — é UX deliberada; o defeito é **como** ela é feita.
- **Não** tratar o `barrier` — é a REQ irmã do item 7.
- **Não** mexer no bind do socket. O defeito é na exibição/abertura, não no `listen`.

## Linked ADR

ADR: <!-- nenhum. Correção de vulnerabilidade; a postura "argv, nunca string de shell" já é implícita
nos ramos corretos do Python. Se a análise mostrar que a postura precisa ser explícita no projeto,
avaliar ADR. -->

## Linked Roadmap

Roadmap:
