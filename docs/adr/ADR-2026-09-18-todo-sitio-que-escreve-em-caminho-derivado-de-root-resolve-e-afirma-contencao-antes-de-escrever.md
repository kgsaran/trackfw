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

### 1. 🔴 O predicado adotado é o que o projeto **já tem e já provou** — não se reinventa

`internal/integrations/manager.go:759`, `rejectSymlinks(root, filename)`:

```go
current := filename
for {
    info, err := os.Lstat(current)
    if err == nil && info.Mode()&os.ModeSymlink != 0 { return fmt.Errorf("refusing symlink path %q", current) }
    if err != nil && !os.IsNotExist(err) { return err }
    if current == root { return nil }
    parent := filepath.Dir(current)
    if parent == current || !beneath(root, current) { return fmt.Errorf("path %q escapes root", filename) }
    current = parent
}
```

Ele faz **as duas coisas**: caminha **todos** os ancestrais até `root` com `Lstat` — fechando o buraco
do "só a folha" — **e** afirma contenção (`beneath`, via `filepath.Rel`) a cada passo, fechando
travessia por `..`.

**Retificação da primeira redação desta ADR:** eu havia escrito *"resolver com `EvalSymlinks` e afirmar
contenção; detectar `ModeSymlink` é condição estreita demais"*. Ao ler a implementação de referência,
duas coisas ficaram claras:

1. **Recusar é mais seguro que resolver.** `EvalSymlinks` + contenção *aceitaria* um symlink que
   resolve para dentro da árvore, e abriria janela de **TOCTOU** entre resolver e escrever.
   `rejectSymlinks` não resolve: recusa link em qualquer nível.
2. **A crítica de "lista de literais" não se aplica aqui.** `ModeSymlink` num laço sobre **todos** os
   ancestrais, somado a `beneath`, é predicado — não catálogo de formas. Junction do Windows e `..`
   já estão cobertos.

🔴 **Escrever a ADR contra a implementação existente teria mandado reimplementá-la pior** — o erro que
o `CLAUDE.md` nomeia como o mais caro desta categoria, e que o ML-3B do #392 cometeu há poucas horas
ao reescrever `readFileForRule` à mão.

### 2. A verificação é **extraída** para um ponto único, não copiada por sítio

`rejectSymlinks` e `beneath` saem de `internal/integrations` para um pacote folha alcançável por todos
os sítios de escrita — provavelmente junto de `internal/pathanchor`, que já existe. **Extração, não
reescrita**: a semântica é a que já está em produção na classe (c). 🔴 Copiar o teste
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
- O predicado cobre formas que ninguém enumerou — junction do Windows, `..` não normalizado — porque
  **caminha todos os ancestrais** e afirma contenção a cada passo, em vez de catalogar formas de link.
- Um ponto único de verificação, em vez de uma guarda por sítio que diverge com o tempo.

**Negativas / aceitas**
- Toca muitos sítios de escrita; o risco de falso-positivo é real, e por isso o braço **(b)** é
  inegociável em cada um.
- Um `Lstat` por ancestral custa syscalls. Aceito: escrita de arquivo já é I/O, a verificação é por
  operação (não por byte), e a classe (c) já paga esse custo em produção sem queixa.
- 🔴 **Recusar é estritamente mais estrito que resolver:** um symlink que aponta para **dentro** da
  árvore também passa a ser recusado. Projetos que dependam disso — `scripts/` apontando para um
  diretório compartilhado, por exemplo — quebram. É o preço de fechar o TOCTOU, e a classe (c) já o
  cobra hoje sem incidente relatado. Se aparecer caso legítimo, a saída é **explícita e nomeada**,
  nunca afrouxamento silencioso do predicado.
