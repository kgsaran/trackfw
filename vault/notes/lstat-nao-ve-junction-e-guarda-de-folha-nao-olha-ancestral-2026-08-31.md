---
title: Lstat não vê junction no Windows, e guarda de folha nunca olha ancestral (em nenhum SO)
tags: [windows, symlink, junction, seguranca, guarda, paridade, gotcha]
date: 2026-08-31
related: [[update-segue-symlink-e-escreve-fora-do-projeto-2026-08-28]]
---

## Sintoma

Toda guarda do projeto que recusa escrever através de link decide assim:

```go
if info.Mode()&os.ModeSymlink != 0 { /* recusa */ }
```

No Windows, **isso não recusa uma junction** — e junction é o caso *fácil* de plantar.

## O que a sonda mediu

`windows-probe.yml`, run [`33338382066`](https://github.com/kgsaran/trackfw/actions/runs/33338382066),
`windows-latest`:

```
lstat-common    →  ModeSymlink=false   ModeIrregular=false   ModeDir=false
lstat-symlink   →  ModeSymlink=true    ModeIrregular=false   ModeDir=false
lstat-junction  →  ModeSymlink=false   ModeIrregular=TRUE    ModeDir=false
stat-junction   →  ModeSymlink=false   ModeIrregular=false   ModeDir=true   (seguindo o link)
```

**O `os.Lstat` do Go classifica junction como `ModeIrregular`, não `ModeSymlink`.**

O detalhe que inverte a intuição de segurança: `os.Symlink` no Windows **exige Developer Mode ou
`SeCreateSymbolicLinkPrivilege`**, enquanto `cmd /c mklink /J` **não exige privilégio nenhum**. A
guarda cobre o caso privilegiado e deixa passar o não-privilegiado — exatamente ao contrário do que
se quer.

## As três classes de guarda — não são a mesma coisa

A primeira leitura ("todas as guardas estão furadas") é **errada**, e classificar mal produz o
roadmap de correção errado. Junction é reparse point **de diretório**: não se planta junction num
caminho de arquivo.

### Classe 1 — guarda que caminha ancestrais: **furada**

`internal/integrations/manager.go:702` `rejectSymlinks` (e os pares em Node/Python). Percorre a
cadeia justamente para recusar link em qualquer ponto dela. Junction num ancestral tem
`ModeSymlink=false` → não é recusada → o caminho segue.

### Classe 2 — guarda de diretório: **o freio existe SÓ no Go**

Esta é a que mais custou. Achado do `hades-tf` na Wave 0, confirmado por leitura direta:

| CLI | Código | Comportamento sobre junction |
|---|---|---|
| Go | `manager.go:582` `removeEmptyAncestors` | `if !info.IsDir() { return nil }` → **para** (`ModeDir=false` para junction) |
| Node | `manager.js:420` `cleanEmpty` | **sem** teste de `isDirectory()`; depende de `readdirSync(dir).length` ser truthy |
| Python | `manager.py:589` `_remove_empty` | **sem** teste de `IsDir` nem de vazio — só `except OSError: return` em volta do `rmdir()` |

O Go para **por acidente**: o teste `!IsDir()` existe para outra finalidade e, como `Lstat` de
junction devolve `ModeDir=false`, ele freia. Node e Python não têm esse teste.

**Precisão sobre o raio de dano (correção do `hades-tf` na barreira final, 2026-08-31).** A primeira
redação desta nota dizia que no Python *"o laço continua subindo e removendo ancestrais"*, sem
qualificar até onde. **A subida é limitada ao `root` gerenciado**, não ao sistema de arquivos do
usuário:

```python
while directory != root and root in directory.parents:   # Python: contenção existe
```
```javascript
if (!rel || rel === '..' || rel.startsWith(`..${path.sep}`) || path.isAbsolute(rel)) return   // Node: idem
```

Ou seja: **a contenção geográfica já existe nos três**. O que falta em Node e Python é o **teste de
tipo**, ao lado do teste de link que já está lá.

**Consequência para o remédio — e é por isso que a precisão importa:** escrever *"adicionar
contenção"* produziria trabalho redundante e provavelmente uma segunda checagem conflitante. O
remédio correto é **"adicionar o teste de tipo ao lado do teste de link já existente"**. No Go é
*tornar intencional um freio que já existe*. Tratar os três como a mesma classe produziria correção
errada em dois dos três CLIs — e descrever mal o defeito produziria o remédio errado nos três.

### Classe 3 — guarda de folha: furada por outra via, e **em todo SO**

`internal/generators/update.go:1869`, `:1894`, `internal/discover/discover.go:268` e pares. Fazem
`Lstat` **apenas na folha**. Mas `Lstat` só deixa de seguir o **último** componente do caminho —
**ancestrais são sempre seguidos**. Logo um symlink (ou junction) num diretório ancestral redireciona
a escrita para fora do projeto sem que nenhuma delas olhe.

**Esta metade não tem nada de Windows.** Vale no Linux, no macOS, hoje. É a mais grave das três e a
que menos se parece com o achado original.

## Por que os gates de paridade não pegam

Os três CLIs concordam entre si: os três decidem por symlink e os três ignoram junction. **Paridade
mede se as implementações concordam, não se o contrato está correto** — um gate que só compara
runtimes é cego para erro cometido igualmente nos três. É o mesmo cego do cabeçalho de aceite
(`barrier-so-casa-cabecalho-de-aceite-em-portugues-2026-08-28`).

## MEDIDO nos 3 runtimes (run `33447191373`, 2026-08-31) — e o resultado inverte a expectativa

| runtime | primitiva | junction | veredito |
|---|---|---|---|
| **Go** | `Lstat().Mode()` | `ModeSymlink=false` **`ModeIrregular=true`** `ModeDir=false` | cego, mas **distinguível** |
| **Node** | `lstatSync()` | **`isSymbolicLink=true`** | **ENXERGA** — guarda já funciona |
| **Python** | `islink` / `S_ISLNK` | `islink=False` `S_ISLNK=False` **`S_ISDIR=True`** | cego **e indistinguível de diretório comum** |

**Eu esperava três cegos e encontrei um vidente.** O libuv mapeia o reparse point para symlink, então
**Node não tem o defeito**. A regra de paridade se inverte: os três divergem, e o Node é o certo.

**Python é o pior caso e não era o previsto** — `S_ISDIR` é *verdadeiro*, então pelas primitivas em
uso a junction é igual a um diretório vazio. Mas existe discriminante, medido no mesmo run:

```
junction:  os.readlink() → '\\?\C:\...\targetdir'        ← resolve
comum:     os.readlink() → [WinError 4390] not a reparse point  ← falha
```

O Python **consegue** distinguir — só não pelas primitivas que escolhemos.

### `rmdir` sobre junction vazia: remove a junction, **não o alvo**

Nos três runtimes: `err=nil/null/None`, junction sumiu, **alvo sobreviveu**. **Não há destruição de
dados.** Isto derruba a formulação alarmista que circulou antes ("sobe removendo diretórios do
usuário"): errada por duas vias independentes — a subida é limitada ao `root`, *e* a remoção não
alcança o alvo.

## Remédio — diferente por runtime, o que só se soube medindo

- **Node** — nada na detecção. Mexer seria corrigir o que não está quebrado.
- **Go** — `ModeIrregular` é o discriminante. Ressalva permanece: no Windows também cobre devices e
  pipes nomeados; `ModeSymlink|ModeIrregular` exige medir falso positivo antes.
- **Python** — `islink`/`S_ISLNK` **não servem**. Usar sucesso de `os.readlink()` ou
  `FILE_ATTRIBUTE_REPARSE_POINT` em `st_file_attributes`. **Trocar a primitiva**, não só adicionar teste.

A severidade cai: sem destruição de dados, contida ao `root`, um runtime já imune. **O defeito real e
universal é a Classe 3** — guarda de folha que não olha ancestral, em todo SO e todo runtime.

## Como foi descoberto

Não por inspeção de código: pela **sonda sob demanda**, na primeira execução depois do merge do #221.
A pergunta *"o `Lstat` do Go marca `ModeSymlink` para uma junction?"* tinha sido feita pelo autor da
issue #216 e **nenhuma suíte de regressão a respondia** — regressão só responde o que alguém já
transformou em asserção. Foi exatamente o caso de uso que justificou construir a sonda.

Vale registrar também o que **quase** apagou a medição: a primeira versão do braço Node criava a
junction com `fs.symlinkSync(..., 'junction')` (libuv) em vez de `cmd /c mklink /J`. Mesmo reparse
tag, conteúdo diferente no `REPARSE_DATA_BUFFER` — teríamos confundido *"o `lstat` do Node diverge"*
com *"o objeto medido é outro"*. **Numa medição comparativa, a variável sob teste tem de ser o
runtime, não a fixture.**

## Rastreamento

- `REQ-2026-08-30-sonda-nao-responde-a-pergunta-7-e-nao-mede-junction-em-node-e-python-a-guarda-de-symlink-pode-estar-furada-nos-3-clis-no-windows.md` (aberta) — a medição.
- `REQ-2026-08-29-namespace-de-agente-nao-declarado-...` (`Done`) — tem nota de correção anexada: o AC12 é verdadeiro no Linux e falso no Windows para junction.
- `docs/seguranca/2026-08-30-modelo-de-ameaca-da-extensao-da-sonda.md` — enumeração completa, inclui `copyPath`/`_copy_path` e o código morto `writeCIWorkflowForce`.
