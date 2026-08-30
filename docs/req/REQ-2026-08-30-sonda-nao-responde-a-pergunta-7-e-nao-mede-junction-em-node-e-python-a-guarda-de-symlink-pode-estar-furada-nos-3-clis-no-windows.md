---
status: Open
date: 2026-08-30
author: "zeus-tf"
adr: "docs/adr/ADR-2026-08-30-ci-de-windows-como-instrumento-de-medicao-job-largo-que-nasce-vermelho-mais-sonda-sob-demanda.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-30-sonda-mede-junction-nos-3-runtimes-e-a-pergunta-7-volta-a-responder.md"
---

# REQ: Sonda não responde a pergunta 7 e não mede junction em Node e Python — a guarda de symlink pode estar furada nos 3 CLIs no Windows

> Date: 2026-08-30 | Status: Open

## Motivation

**A sonda pagou o próprio custo na primeira execução pós-merge** (run `33338382066`, `main`) e o que
ela devolveu invalida uma premissa que já tratávamos como fechada.

### O que a sonda mediu

```
lstat-symlink   →  ModeSymlink=true    ModeIrregular=false   ModeDir=false
lstat-junction  →  ModeSymlink=false   ModeIrregular=TRUE    ModeDir=false
stat-junction   →  ModeDir=true   (seguindo o link)
```

**O `os.Lstat` do Go não marca uma junction como `ModeSymlink`; marca como `ModeIrregular`.** E a
junction é criada com `cmd /c mklink /J`, que **não exige privilégio algum** — ao contrário de
`os.Symlink`, que exige Developer Mode. A consequência é perversa: no Windows a guarda captura o
caso que exige privilégio e deixa passar o que não exige.

### O que isso quebra — e o que NÃO quebra

A primeira leitura ("as nove guardas estão furadas") está errada, e a forma correta muda a
severidade. **Junction é reparse point de diretório**: não se planta uma junction num caminho de
arquivo. Separando:

| Guarda | Alvo | Situação |
|---|---|---|
| `internal/integrations/manager.go:702` `rejectSymlinks` | caminha ancestrais | 🔴 **Furada.** É a única guarda que percorre a cadeia justamente para recusar link em qualquer ponto dela. Junction num ancestral tem `ModeSymlink=false`, não é recusada, o caminho segue. |
| `internal/integrations/manager.go:582` `removeEmptyAncestors` | diretório | 🟡 **Salva por acidente.** `Lstat` de junction devolve `ModeDir=false`, então cai no `if !info.IsDir() { return nil }` e para. Funciona pelo motivo errado — qualquer refactor que troque a ordem dos testes reabre o buraco. |
| `internal/generators/update.go:1869`, `:1894`, `internal/discover/discover.go:268` | arquivo (folha) | 🔴 **Furadas por outra via, e não só no Windows.** Fazem `Lstat` apenas na folha. `Lstat` só deixa de seguir o **último** componente do caminho; ancestrais são **sempre** seguidos. Logo um symlink — ou uma junction — num diretório ancestral redireciona a escrita para fora do projeto sem que nenhuma delas olhe. **Esta metade é independente de plataforma.** |

### As duas coisas que ainda não sabemos — e por isso esta REQ existe

1. **A pergunta 7 da sonda falhou** (step 19, run `33338382066`). Não é "a sonda rodou com sucesso":
   uma de sete perguntas voltou sem resposta, e é a irmã desta — *o que um `git checkout`
   materializa para um symlink versionado no Windows*. Causa: em PowerShell a **vírgula constrói
   array**, então `git update-index --cacheinfo 120000,$blob,mylink` chegou ao git como três
   argumentos separados (`error: option 'cacheinfo' expects <mode>,<sha1>,<path>`). Salvou-se um
   dado: `core.symlinks = true` no runner.
2. **Node e Python não foram medidos.** Há hipótese — libuv mapeia reparse points para symlink, o
   que faria `lstatSync().isSymbolicLink()` pegar junction; `os.path.islink` do Python
   provavelmente não pega — mas **hipótese não decide nada**. Sob a regra dura de paridade,
   *"defeito só do Go"* e *"divergência entre os três"* são correções diferentes, com roadmaps
   diferentes. Medir primeiro é o que impede escrever o roadmap errado.

### Consequência de governança que esta REQ registra

A `REQ-2026-08-29-namespace-de-agente-nao-declarado-em-agents-fica-invisivel-e-o-validate-reporta-limpo-sem-olhar.md`
está **`Done`** com o **AC12** afirmando *"a enumeração **não segue symlink**. Verificável nos 3
CLIs"*. Essa afirmação é **verdadeira no Linux e falsa no Windows para junction**. Pela regra do
projeto, AC marcado que contradiz o medido é bug no artefato — não rodapé. Esta REQ exige a nota de
correção **na REQ original**, sem reabri-la nem reescrever seu histórico.

### Por que medir antes de corrigir

O candidato a correção já saiu da própria sonda — testar `ModeSymlink|ModeIrregular` em vez de só
`ModeSymlink`. Mas `ModeIrregular` no Windows também cobre devices, pipes nomeados e outros reparse
points; adotá-lo sem medir arrisca trocar um falso negativo por um falso positivo que quebra uso
legítimo. **Esta REQ não corrige guarda nenhuma** — ela produz o número que decide a correção.

## Acceptance Criteria

- [ ] **AC1** — A pergunta 7 volta a responder. `git update-index --cacheinfo` recebe **um único
      argumento**, não um array do PowerShell. O step conclui `success` e imprime o que ficou em
      disco após o `checkout` de um symlink versionado.
- [ ] **AC2** — 🔴 **Falsificação da correção da AC1**: provar que o argumento chega íntegro, não
      apenas que o comando parou de errar. Um `git update-index` que falhasse por outro motivo e
      fosse silenciado também faria o step passar.
- [ ] **AC3** — A sonda passa a medir **junction em Node**: `lstatSync()` sobre junction, imprimindo
      `isSymbolicLink()`, `isDirectory()` e `isFile()` **crus**, ao lado do mesmo trio para um
      symlink real e para um arquivo comum — o mesmo formato comparativo que o braço Go já usa.
- [ ] **AC4** — A sonda passa a medir **junction em Python**: `os.path.islink()`, `os.lstat().st_mode`,
      `stat.S_ISLNK()` e `os.readlink()` (com o erro, se levantar) sobre junction, symlink e arquivo
      comum.
- [ ] **AC5** — **Saída comparável entre os 3 runtimes**: a sonda imprime uma tabela final
      `runtime × (arquivo | symlink | junction)` que permite ler a divergência sem cruzar logs à mão.
      É o artefato que a REQ de correção vai citar.
- [ ] **AC6** — 🔴 **A sonda continua sem veredito.** Nenhuma das perguntas novas emite pass/fail nem
      `exit 1` por causa do *valor* medido. Sonda que ganha veredito vira job de regressão disfarçado
      — o AC6 do ADR é explícito e inviolável.
- [ ] **AC7** — **Nota de correção na REQ-2026-08-29**, registrando que o AC12 é verdadeiro no Linux
      e falso no Windows para junction, com link para o run que mediu. Sem reabrir a REQ, sem
      reescrever o AC original.
- [ ] **AC8** — Nota de vault sobre `Lstat`/junction/`ModeIrregular`, com a separação
      guarda-de-diretório vs guarda-de-folha-sem-ancestral. Critério: outro agente perderia mais de
      dez minutos amanhã sem ela.
- [ ] **AC9** — A sonda roda em `windows-latest` e o log traz as respostas novas. **Verificável só
      pós-merge** (`workflow_dispatch` exige o workflow na branch default) — registrado como
      verificação diferida, não marcado como satisfeito antes de existir.
- [ ] **AC10** — `actionlint` limpo; `make quality` verde; nenhum job de `quality.yml` alterado.

## Negative Scope — o que esta REQ NÃO faz

- **Não corrige guarda nenhuma.** Nem `rejectSymlinks`, nem as guardas de folha, nem
  `removeEmptyAncestors`. A correção é a REQ seguinte, e depende do número que esta produz.
- **Não adota `ModeSymlink|ModeIrregular`.** É candidato, não decisão — carece da medição de falso
  positivo em devices e pipes.
- **Não reabre a REQ-2026-08-29** nem altera seu AC12. Só anexa nota de correção.
- **Não toca `quality.yml`** nem as duas camadas de regressão. Esta REQ é só sobre a sonda.
- **Não trata a fuga por ancestral independente de plataforma** (a metade Linux do achado). É defeito
  real e mais grave, mas é escopo da REQ de correção — misturar as duas faria a medição competir com
  a correção no mesmo roadmap.

## Linked ADR

ADR: `ADR-2026-08-30-ci-de-windows-como-instrumento-de-medicao-job-largo-que-nasce-vermelho-mais-sonda-sob-demanda.md`
— decisão 3 (sonda sob demanda) e AC6 (sonda não é o job de regressão).

## Linked Roadmap

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-30-sonda-mede-junction-nos-3-runtimes-e-a-pergunta-7-volta-a-responder.md`
