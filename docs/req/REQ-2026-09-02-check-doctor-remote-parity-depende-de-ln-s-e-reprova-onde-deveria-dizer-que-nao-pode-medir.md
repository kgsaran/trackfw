---
status: Open
date: 2026-09-02
author: "lourivalgarciajunior"
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-02-gate-do-doctor-remoto-usa-wrapper-em-vez-de-symlink.md"
---

# REQ: `check-doctor-remote-parity` depende de `ln -s` e reprova onde deveria dizer que não pode medir

> Date: 2026-09-02 | Status: Open

## Motivation

`scripts/check-doctor-remote-parity.sh` **não executa num Windows comum**:

```
ln: failed to create symbolic link '.../runtimebin/python3': Permission denied
```

Causa isolada por execução:

```
command -v python3       →  .../WindowsApps/python3   (109 bytes → C:/Program Files/WindowsApps/)
ln -s para /bin/ls       →  criou
ln -s para esse python3  →  Permission denied
```

`python3` é o **App Execution Alias da Microsoft Store** — o default de quem instala Python pela
Store. O MSYS o enxerga como symlink, e sem `winsymlinks:nativestrict` o `ln -s` **copia** o alvo;
copiar um reparse point de `WindowsApps` é negado pelo sistema.

**Não é sobre Developer Mode.** O `ln -s` funciona nesta máquina para um alvo comum — o que falha é
o alvo específico. Um `git config` ou um flag de ambiente não resolvem.

## O ponto que importa mais que a portabilidade

O gate emite **reprovação**, não *"não pode ser avaliado"*. O commit `9a6fc81`, do mesmo lote que
introduziu este gate, estabelece exatamente essa distinção na AC4 do `barrier`:

> distinguir "gate reprovou" de "gate NÃO PODE SER AVALIADO". Hoje ambos viram falha. Se o `sh` não
> existe, o resultado não é "a wave não passou", é "não deu para medir". Confundir os dois é a mesma
> classe que atravessou esta sessão inteira: tratar ausência de medição como medição negativa.

O critério existe e é da casa. Este gate nasceu sem ele — nenhuma ocorrência de `skip` ou de
detecção de condição no script.

## A correção não precisa escolher entre pular e medir

O `RUNTIME_BIN` existe para uma garantia única, declarada no próprio comentário do script: **um PATH
onde só `node`, `python3` e `git` são resolvíveis**, para que um `gh` real instalado na máquina não
vaze para um cenário que precisa não ter nenhum. O symlink é o *meio*, não o requisito.

Um wrapper de duas linhas entrega a mesma garantia sem privilégio nenhum. Medido:

```
PATH=<runtimebin> python3 --version  →  Python 3.12.4     resolve
PATH=<runtimebin> node --version     →  command not found  não vaza
PATH=<runtimebin> gh --version       →  command not found  não vaza
```

## Acceptance Criteria

- [ ] **AC1** — `RUNTIME_BIN` é populado sem `ln -s`, por wrapper com **shebang absoluto** — mesmo
      motivo já registrado no script para o stub do `gh`: não depender de PATH para resolver o
      próprio interpretador.
- [ ] **AC2** — 🔴 **A garantia de isolamento é preservada e verificada, não presumida.** Com o
      `RUNTIME_BIN` como PATH único: `node`, `python3` e `git` resolvem; **qualquer outro executável
      instalado na máquina não resolve** — em particular `gh`, que é a razão de o isolamento existir.
- [ ] **AC3** — 🔴 **Falsificação nas duas direções.** (a) o gate, que hoje morre no `ln -s`,
      atravessa e chega ao veredito num Windows onde `python3` é o alias da Store; (b) **controle** —
      num ambiente onde o `ln -s` sempre funcionou (Linux/macOS do CI), o gate continua dando o mesmo
      veredito de antes, ou seja, a troca não alterou o que ele mede.
- [ ] **AC4** — O comentário no código registra **por que não é symlink**, com o alvo medido. Sem
      isso, a próxima pessoa "simplifica" de volta para `ln -s`.

## Negative Scope

**Não introduz skip por condição neste gate.** Seria a resposta certa se a medição fosse impossível,
e ela não é — o wrapper mede. Pular quando dá para medir é perder cobertura, e a AC4 do `barrier`
distingue os dois casos justamente para não confundi-los.

**Não varre** os outros scripts atrás de `ln -s`. É plausível que existam; não medi, e não afirmo.

**Não corrige o segundo defeito deste mesmo gate**, medido no caminho e descrito abaixo. Ele tem
mecanismo próprio, e a correção envolve uma decisão de desenho sobre o shim de Windows que é sua,
não minha.

## Segundo achado, medido — o stub de `gh` é invisível para o Go no Windows

Depois da correção do `ln -s`, o gate atravessa e **chega ao veredito** — e aí 7 dos 33 cenários
reprovam, todos pela mesma causa:

```
[not-evaluated] branch-protection
  remedy: install the GitHub CLI (gh) to evaluate branch protection remotely
```

O gate cria o stub como `gh`, **sem extensão**. O `bash` executa pelo shebang; o
`exec.LookPath` do Go **não o enxerga** — no Windows ele só resolve nomes com extensão do
`PATHEXT`. Medido:

```
LookPath("gh")      = ...in\gh.cmd     err=<nil>      (só depois de eu criar o .cmd)
LookPath("gh.cmd")  = ...in\gh.cmd     err=<nil>
```

Antes de existir o `.cmd`, `LookPath("gh")` não achava nada — e o doctor reportava
`not-evaluated` para todo cenário.

**Independente da correção do `ln -s`**, verificado rodando o binário Go sozinho, com o stub no
PATH e sem wrapper nenhum envolvido: reproduz igual.

É a mesma classe do defeito principal: o gate **reprova** onde a resposta correta é *"não deu para
medir neste ambiente"*.

## Linked ADR
<!-- Correção de harness, tratamento único; sem decisão de arquitetura a registrar. -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/wip/ROADMAP-2026-09-02-gate-do-doctor-remoto-usa-wrapper-em-vez-de-symlink.md
