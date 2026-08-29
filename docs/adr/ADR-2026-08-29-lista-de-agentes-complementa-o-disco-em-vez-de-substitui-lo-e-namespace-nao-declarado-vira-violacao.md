---
status: Accepted
date: 2026-08-29
author: "trackfw_architect (Zeus)"
---

# ADR: A lista `agents:` complementa o disco em vez de substituí-lo, e namespace não declarado vira violação

> Date: 2026-08-29 | Status: Accepted

## Context

Reportado por um agente trabalhando no projeto **cmdb** — um projeto consumidor, não este
repositório. Sintoma visível: `trackfw roadmap move` falhava com
`not found in any state directory` para um arquivo **que existe em disco**.

A causa é o modo `roadmap_namespacing: by_agent`. Medido em
`internal/validator/validator.go:1017-1037`:

```go
if cfg.RoadmapNamespacing == config.NamespacingByAgent {
    agents := cfg.Agents
    if len(agents) == 0 {                    // só cai no disco se a lista estiver VAZIA
        entries, err := os.ReadDir(cfg.RoadmapDir)
        ...
    }
    for _, agent := range agents {           // senão, SÓ o que está declarado
        dirs = append(dirs, cfg.RoadmapDir+"/"+agent+"/"+state)
    }
```

**A lista `agents:` do `trackfw.yaml` substitui o disco.** Diretório que existe e não está declarado
é invisível — sem aviso, sem erro. No cmdb, `agents:` lista sete agentes e **não inclui `zeus`**;
`docs/roadmaps/zeus/` e `docs/requisições/zeus/` estavam fora de tudo.

**A consequência grave não é o `move` falhar — é o `validate` passar.** Ele reportava
`No violations found` sobre um conjunto que nunca enumerou, incluindo uma REQ de ratificação criada
naquela semana. O `move` foi a **única** manifestação visível, e é por isso que o defeito foi
encontrado por acaso: as outras leituras degradam em silêncio.

*Uma ferramenta de governança que dá atestado de saúde de artefatos que não abriu é pior que uma que
quebra — a quebra alguém percebe.*

**Incentivo perverso do desenho atual:** `agents:` **vazio** é mais seguro que `agents:`
**incompleto**. Vazio cai no disco e enxerga tudo; incompleto parece configurado e esconde.

**Escala do problema, medida:** a regra "lista declarada substitui disco" está duplicada em **6
funções independentes** só no `validator.go` (`validateWIPLimit:221`, `GetStatus:912`,
`resolveStateDirs:1020`, `resolveREQFiles:1071`, `validateFolderStatusCoherence:1959`,
`validateFilenameUniqueness:2036`), e o modo `by_agent` aparece em **9 arquivos** no Go, **11**
sítios no Node e **24** no Python. `roadmap move`, `validate`, `serve` e `context` resolvem o
diretório cada um por conta própria. **Corrigir um não corrige o defeito** — é a mesma forma do
ML-2G da REQ anterior, onde o caminho de alvos foi consertado e o caminho nu continuou quebrado.

## Decision

**1. `agents:` COMPLEMENTA o disco.** A enumeração passa a ser a **união** entre a lista declarada e
os diretórios encontrados em `roadmap_dir`/`req_dir`. Nada em disco fica invisível, em nenhum modo,
por nenhuma configuração.

**2. Namespace em disco e não declarado vira VIOLAÇÃO**, nomeando o diretório e dizendo como
resolver (acrescentar a `agents:` no `trackfw.yaml`). Não é aviso: é defeito de configuração que
escondeu artefatos de governança, e a correção é de uma linha.

A união (decisão 1) é o que torna a violação segura de emitir — o usuário não perde nada enquanto
não corrige, ele apenas fica sabendo. Emitir violação **sem** a união seria manter a cegueira e
ainda reclamar dela.

**3. Um resolvedor canônico por runtime**, consumido por todos os pontos que hoje resolvem sozinhos.
A duplicação é o que tornou o defeito onipresente e é o que faria a correção ficar pela metade.

**4. Vale para `req_dir` também, não só `roadmap_dir`.** No cmdb as duas árvores estavam cegas.

**5. A enumeração de diretórios NÃO segue symlink — decisão acrescentada em 2026-08-29, após a
Wave 0.** O `hades-tf` achou, e eu reproduzi com os três binários: um namespace que é **symlink**
apontando para fora do projeto faz o `roadmap move` **escrever fora da árvore**.

```
docs/roadmaps/evil → /fora/do/projeto

go     escreveu fora?  não      (os.ReadDir + entry.IsDir() não segue symlink)
node   escreveu fora?  SIM      (fs.statSync().isDirectory() segue)
py     escreveu fora?  SIM      (os.path.isdir() segue)
```

O Go é imune **por acidente** de qual primitiva usa, não por decisão.

**E aqui está o motivo de isso bloquear a Wave 1:** hoje o escape é **condicionado à configuração** —
só dispara quando `agents:` está vazia, porque só então o disco é lido. **A união torna a leitura de
disco incondicional**, e com ela o escape passa a valer para **todo projeto `by_agent`**. Implementar
a decisão 1 sem esta seria transformar um buraco estreito em universal.

É a segunda fuga por symlink em dois dias — a primeira foi o `update`/`discover` escrevendo fora do
projeto (`vault/notes/update-segue-symlink-e-escreve-fora-do-projeto-2026-08-28.md`) — e a segunda vez
que ampliar cobertura ampliou superfície. *Cobertura maior é superfície maior* deixou de ser
observação e virou regra de desenho aqui.

**A enumeração usa checagem que não segue link**, nos 3 runtimes: `os.ReadDir` + `entry.IsDir()` em
Go (preservar, **não** "simplificar" para `os.Stat`), `fs.readdirSync(dir, {withFileTypes: true})` +
`dirent.isDirectory()` em Node, e `os.scandir` + `is_dir(follow_symlinks=False)` em Python.

## Consequences

**Positivas**
- Nenhum artefato de governança pode ficar invisível por configuração. O `validate` volta a
  responder sobre o que existe, não sobre o que foi declarado.
- O `roadmap move` passa a achar o que está em disco.
- A violação empurra a configuração para a realidade, em vez de deixar as duas divergirem em
  silêncio.

**Negativas e riscos aceitos**
- **Artefatos antes invisíveis passam a ser avaliados, e podem revelar violações reais.** Um projeto
  que hoje reporta limpo pode passar a reportar problemas — que sempre existiram e nunca foram
  vistos. É correção, não regressão, mas vai surpreender. Precisa estar claro na mensagem.
- Diretório espúrio em `roadmap_dir` (resto de migração, pasta temporária) passa a ser enumerado e a
  gerar violação de namespace não declarado. É ruído legítimo — o remédio é apagar a pasta ou
  declará-la —, mas é atrito novo.
- Um resolvedor canônico por runtime é refatoração ampla em código que hoje funciona para quem tem
  `agents:` completo. O risco de regressão é real e exige gate de falsificação nas duas direções.

## Alternatives Considered

**Só fazer a união, sem violação.** Corrige a cegueira e é a mudança menor. Rejeitada: deixa a
configuração errada para sempre, sem ninguém saber. `trackfw.yaml` e disco continuariam divergindo,
e a próxima ferramenta que confiar na lista repete o defeito.

**Só emitir violação, sem união.** Corrige a configuração e mantém a cegueira até alguém agir — e
enquanto isso o `validate` segue reportando limpo sobre o que não viu. É a pior das três.

**Fazer o `agents:` obrigatório em modo `by_agent`, falhando se ausente.** Não resolve: o problema
não é lista ausente, é lista **incompleta** — que é exatamente o que parece correto.

**Deprecar `agents:` e sempre usar o disco.** Tentador pela simplicidade. Rejeitada: a lista tem uso
legítimo de ordenação e de exibição no `serve`, e removê-la é mudança de contrato de configuração
para todos os projetos, sem necessidade.
