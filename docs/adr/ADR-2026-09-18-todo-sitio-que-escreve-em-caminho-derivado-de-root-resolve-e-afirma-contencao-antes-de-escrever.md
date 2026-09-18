---
status: Accepted
date: 2026-09-18
author: "Zeus (arquiteto) / KG (decisão)"
---

# ADR: todo sítio que escreve em caminho derivado de `root` resolve e afirma contenção antes de escrever

> Date: 2026-09-18 | Status: Accepted

**REQ:** `REQ-2026-08-31-guarda-de-folha-faz-lstat-so-no-ultimo-componente-e-nunca-inspeciona-ancestral...`

## Context

O produto escreve **fora da árvore do projeto**, em dois caminhos independentes, ambos reproduzidos
pelo arquiteto em 2026-09-18 com o binário da `main`.

### Sítio A — symlink em diretório **ancestral**

```bash
mkdir -p /fora/out && ln -s /fora/out .github && ln -s /fora/out scripts
trackfw discover --init
```

Resultado: **6 arquivos** gravados em `/fora/out` — `trackfw-validate.sh`,
`trackfw-attention-signal.sh`, `trackfw-attention-cleanup.sh`, `trackfw-credential-guard.sh`,
`trackfw-git-branch-guard.sh` e `workflows/trackfw-validate.yml`. **Nenhum aviso.**

A guarda existente faz `os.Lstat` na **folha**. 🔴 **`Lstat` só deixa de seguir o último componente;
ancestrais são sempre seguidos.** A folha não é symlink, a checagem passa, a escrita sai da árvore.

### Sítio B — symlink na **folha**

```bash
ln -s /fora/vitima.md docs/roadmaps/backlog/ROADMAP-isca.md
trackfw roadmap move ROADMAP-isca wip        # → ✓ moved
```

O `status:` de `/fora/vitima.md` foi reescrito de `PRESERVAR` para `wip`.

### A causa é uma só

Os dois sítios são faces do mesmo defeito: **o produto resolve um caminho e escreve sem afirmar que
o destino está contido na árvore.** Num, o symlink está no ancestral e a guarda olha a folha; no
outro, está na folha e não há guarda alguma.

Pelo teste da Regra Dura de Causa Raiz — *"se eu corrigir esta causa, exatamente estas falhas fecham
e nenhuma outra"* — corrigir "afirmar contenção antes de escrever" fecha **os dois**.

### A superfície é larga

Varredura por primitivos de escrita (`os.WriteFile`, `os.Rename`, `MkdirAll`) em `internal/`:
**12+ arquivos**, entre eles `discover`, `generators/roadmap`, `generators/agentfiles`,
`generators/update`, `integrations/manager`, `identity`, `config`. A enumeração real é a **Wave 0**.

## Decision

### 1. O predicado é **resolver-e-afirmar-contenção**, não detectar symlink

Antes de escrever em caminho derivado de `root`: **resolver o destino** (`filepath.EvalSymlinks` ou
equivalente no diretório-pai já materializado) e **afirmar que o resultado está sob a raiz real do
projeto**. Recusar caso contrário.

🔴 **Detectar `ModeSymlink` é a condição estreita demais** que a `ADR-2026-08-22` nomeia: a lista de
formas a detectar nunca fecha — symlink de folha, de ancestral, junction do Windows, hardlink,
bind mount, `..` em caminho não normalizado. **Contenção é um predicado**; detecção de link é uma
lista de literais.

### 2. A verificação vive num ponto único, não copiada por sítio

A função de contenção mora num pacote alcançável por todos os sítios de escrita. 🔴 Copiar o teste
por sítio é o defeito que a REQ do **#392** acabou de fechar em outra superfície — quatro dialetos de
"este ML está concluído?" que discordavam. Não repetir a forma em outra superfície na semana seguinte.

### 3. A recusa é audível e nomeia o caminho

Mensagem em stderr com o caminho e o motivo. Silêncio vira *"o update não atualizou meu arquivo e não
disse nada"* — e um guard que confunde o usuário é um guard que o usuário desliga
(`ADR-2026-08-17`).

### 4. 🔴 Falsificação nas duas direções, obrigatória por sítio

- **(a)** com symlink apontando para fora, a escrita é recusada e **nada** é criado fora da árvore;
- **(b)** **controle**: a operação legítima, sem link algum, **continua funcionando**.

Sem **(b)**, trocamos um buraco por uma quebra. É o mesmo par que o #392 exigiu em cada gate.

### 5. A REQ irmã do `roadmap move` é **absorvida**, não tratada em paralelo

`REQ-2026-08-30-roadmap-move-segue-symlink...` passa a `Superseded`, apontando para esta. Mesma
causa, mesma REQ, mesmo PR — Regra Dura.

🔴 Isto também corrige um erro de processo do arquiteto: no #392 varri os **issues** abertos e **não**
as **REQs** abertas, e esta REQ irmã tocava o `MoveRoadmap` que aquele ML-3A reescreveu. A varredura
de REQs abertas passa a ser parte do protocolo, não cortesia.

## Consequences

**Positivas**
- Fecha escrita arbitrária fora do projeto, que é a classe mais grave medida no corpus hoje.
- O predicado cobre formas que ninguém enumerou — junction, hardlink, `..` não normalizado — porque
  afirma o destino em vez de catalogar a origem.
- Um ponto único de verificação, em vez de uma guarda por sítio que diverge com o tempo.

**Negativas / aceitas**
- Toca muitos sítios de escrita; o risco de falso-positivo é real, e por isso o braço **(b)** é
  inegociável em cada um.
- `EvalSymlinks` custa syscalls. Aceito: escrita de arquivo já é I/O, e a verificação é por operação,
  não por byte.
- Projetos que hoje **dependem** de um symlink legítimo (ex.: `scripts/` apontando para um diretório
  compartilhado) passam a ser recusados. **Precisa ser medido na Wave 0** e, se real, tratado com uma
  saída explícita — nunca com afrouxamento silencioso do predicado.
