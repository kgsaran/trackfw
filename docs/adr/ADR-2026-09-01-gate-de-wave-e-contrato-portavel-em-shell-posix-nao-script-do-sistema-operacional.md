---
status: Accepted
date: 2026-09-01
author: "zeus-tf"
---

# ADR: Gate de wave é contrato portável em shell POSIX, não script do sistema operacional

> Date: 2026-09-01 | Status: Accepted

## Contexto

**Item 7 do issue #216**, reportado por `lourivalgarciajunior`: o mesmo bloco `Gates da wave:` produz
**vereditos diferentes conforme o CLI que executa o `barrier`**.

Medido nos três:

| runtime | chamada | shell efetivo |
|---|---|---|
| Go | `exec.Command("sh", "-c", command)` (`internal/commands/barrier.go:729`) | **`sh` POSIX, em qualquer SO** |
| Node | `spawnSync(cmd, { shell: true })` (`npm/src/commands/barrier.js:561`) | shell do SO — `cmd.exe` no Windows |
| Python | `subprocess.run(cmd, shell=True)` (`pypi/trackfw/commands/barrier.py:582`) | shell do SO — `cmd.exe` no Windows |

**O Go é o divergente — e é o único consistente entre plataformas.** Um "2 contra 1" ingênuo
concluiria que o Go está errado.

## A decisão não é de preferência: é o que os gates reais escrevem

Varredura de **todos** os blocos `Gates da wave:` dos roadmaps do projeto — **83 comandos**:

| idioma | ocorrências |
|---|---|
| `grep` / `sed` / `awk` | 35 |
| `test` / `[` | 14 |
| negação com `!` | 8 |
| `&&` / `\|\|` | 3 |
| substituição `$( )` | 3 |
| pipe `\|` | 2 |

**Nenhum desses existe no `cmd.exe`.** `test -f x` não é comando do Windows; `! grep -q` não é
sintaxe dele; `$(...)` não é substituição ali.

Isso reenquadra o defeito. No Windows, Node e Python **não avaliam o gate de forma diferente — eles
falham em avaliá-lo**. O Go é o único que executa o que os roadmaps de fato contêm.

## Decisão

**O bloco `Gates da wave:` é um contrato portável escrito em shell POSIX.** Os três CLIs executam-no
com `sh -c`, em qualquer sistema operacional. O comportamento do Go passa a ser o dos três.

### Por que não a alternativa oposta

Declarar *"gate é script do SO"* tornaria **todo roadmap existente inválido no Windows** e faria o
conteúdo do gate depender de quem o executa — destruindo a propriedade que dá sentido ao artefato:
**o mesmo roadmap, avaliado em qualquer máquina, dá o mesmo veredito.** Um gate que muda de
significado conforme o SO não é critério de aceite; é sugestão.

### Consequência assumida: `sh` vira pré-requisito no Windows

Já era, de facto, para quem usa o CLI Go. Esta decisão **torna explícito** o que o projeto já
dependia, e alinha com a postura de exigir shell POSIX documentada para o Windows.

**O `sh` ausente tem de falhar com mensagem que nomeia o remédio** — não com erro de baixo nível. Um
usuário Windows sem Git Bash precisa ler *"instale um shell POSIX"*, não *"exec: sh: not found"*.

## Consequências

- `barrier.js` e `barrier.py` passam a invocar `sh -c`, deixando de usar o shell do SO.
- Gates existentes continuam válidos **sem edição** — é o ponto: eles já são POSIX.
- Um gate que dependa de `cmd.exe` deixa de funcionar. **Nenhum existe hoje** (medido acima).
- O contrato entra em `docs/cli-parity.md`, com gate que impeça regressão para `shell: true`.

## Alternativas descartadas

**Restringir a sintaxe a um subconjunto portável entre `sh` e `cmd.exe`.** Esse subconjunto é
praticamente vazio — não inclui `test`, `grep`, negação nem substituição. Invalidaria **83 de 83**
comandos existentes.

**Detectar o SO e adaptar a sintaxe.** Faria o `barrier` reinterpretar o conteúdo do artefato, que é
o oposto de tratá-lo como contrato — e tornaria o veredito dependente de heurística.

## Rastreamento

REQ: `REQ-2026-09-01-mesmo-gate-de-wave-da-vereditos-diferentes-conforme-o-cli-que-executa-o-barrier.md`
