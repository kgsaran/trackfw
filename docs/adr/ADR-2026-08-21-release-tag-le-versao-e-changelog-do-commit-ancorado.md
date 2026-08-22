---
status: Accepted
date: 2026-08-21
author: "Zeus (Arquiteto)"
---

# ADR: `release tag` lê versão e `CHANGELOG` do commit ancorado, não do working tree

> Date: 2026-08-21 | Status: Accepted

## Context

O `ADR-2026-08-19/Emenda 1` ancorou o **commit-alvo** da tag no forge. As Pré-condições 3 e 4 do
`release tag` continuaram lendo o **working tree** (`internal/commands/release.go:302-329`, via
`deps.readFile`): os 4 arquivos de versão e o `CHANGELOG.md`.

O argumento que torna isso urgente é do `hades-tf`, e é contraintuitivo: **corrigir o commit-alvo
tornou a mensagem forjada mais crível**. Antes, uma tag suspeita podia apontar para um commit
estranho — um sinal. Agora ela aparece pendurada num commit real do tip da branch padrão, com a
mensagem que o atacante escreveu, assinada pela credencial do usuário.

A correção de um vetor ampliou a credibilidade do outro.

### O problema que quase levou ao desenho errado

A formulação óbvia é *"comparar o conteúdo local com o de `origin/<default>`"*. Ela **quebra o fluxo
normal de release**: o `CHANGELOG.md` local está **legitimamente à frente** do forge durante o PR de
bump — o mesmo PR que bumpa a versão acrescenta a seção, e a tag é criada **depois** do merge.

Uma verificação assim tornaria o comando inutilizável no próprio fluxo que ele existe para servir.
Falso-positivo aqui não irrita: **paralisa**.

## Decision

**Não comparar local com remoto. Não ler o local.**

O comando **já resolve** o sha do commit-alvo a partir do forge. Versão e `CHANGELOG` passam a ser
lidos **daquele commit**, não do working tree:

```
sha := <do forge>                       (ja implementado, Emenda 1 do ADR-2026-08-19)
git show <sha>:CHANGELOG.md
git show <sha>:internal/version/version.go   (e os demais arquivos de versao)
```

### Por que isto é ancoragem de verdade, e não leitura local disfarçada

Objetos git são **endereçados por conteúdo**. Dado um sha, o conteúdo é criptograficamente
determinado — não é possível forjar um objeto local que case com um sha vindo do forge. A leitura
acontece no disco, mas **a autoridade é o sha**, e o sha vem do forge.

É a mesma propriedade que o `ADR-2026-08-19` usou para o commit-alvo, aplicada ao conteúdo.

### Por que resolve o falso-positivo do PR de bump

O commit-alvo é o **tip pós-merge**. A seção do `CHANGELOG` e o bump dos arquivos de versão **já
estão nele** — foi o merge que os colocou lá. Não há divergência a tolerar porque não há comparação:
lê-se a fonte certa desde o início.

O caso que quebraria o desenho ingênuo **deixa de existir**, em vez de precisar de exceção.

### Consequências aceitas, declaradas

- **O working tree deixa de influenciar a tag.** Editar o `CHANGELOG` local e taggear sem commitar
  passa a **não** funcionar. É o ponto — mas é mudança de comportamento, e quem tiver esse hábito
  vai encontrar uma recusa.
- **O objeto precisa estar presente localmente.** Após o `git fetch origin --prune` que o comando já
  faz, estará no caminho normal. Ausente, a saída correta é **recusar nomeando o que falta**, nunca
  cair para o working tree — cair seria desfazer a ancoragem em silêncio.
- **Não elimina agente induzido.** Coerente com o `ADR-2026-08-12`: quem controla o repositório pode
  commitar um `CHANGELOG` hostil e mergear. O ganho é que a adulteração passa a exigir **um commit
  mergeado** — visível, atribuível e revisável — em vez de uma edição local invisível.

## Consequences

**Positivas**
- Versão e mensagem deixam de ser determinadas por conteúdo local editável.
- **Nenhuma chamada de API nova** — o sha já é resolvido, e a leitura é local por conteúdo.
- Funciona offline depois do fetch, ao contrário de uma verificação via API.
- O fluxo de release não muda para quem o segue.

**Negativas / riscos**
- Mudança de comportamento para quem taggeava a partir do working tree.
- Um `git show` que falhe por objeto ausente precisa produzir recusa clara, não confusão.

## Alternatives Considered

- **Comparar local com `origin/<default>`** — rejeitada: quebra o PR de bump, que é o fluxo normal.
  Exigiria exceção, e exceção em verificação de segurança é o que se aprende a ignorar.
- **Buscar o conteúdo pela API do forge** (`GET /repos/{o}/{r}/contents/...`) — rejeitada: mesma
  garantia, mais chamadas de rede, e quebra o uso offline. O endereçamento por conteúdo do git já
  entrega a ancoragem de graça.
- **Só avisar em vez de recusar** — rejeitada: aviso em caminho que **publica** é o pior dos dois
  mundos; quem ignora publica assim mesmo.
- **Exigir working tree limpo e confiar nele** — rejeitada: a Pré-condição 1 já exige árvore limpa, e
  isso não impede que o conteúdo commitado localmente difira do que o forge tem. Árvore limpa não é
  ancoragem.

---

## Emenda 1 (2026-08-21) — `git show` honra `refs/replace/`, e isso furava a âncora

> Esta ADR está `Accepted`. A emenda **acrescenta**; nada acima foi reescrito.

A barreira do ML-3A **bloqueou**, e o achado invalida o argumento central escrito acima.

O ADR afirma: *"objetos git são endereçados por conteúdo. Dado um sha, o conteúdo é
criptograficamente determinado."* Isso é **verdadeiro para o object store** e **falso para
`git show`** — que passa pela camada de substituição de objetos.

Reproduzido por mim:

```
sha ancorado (do forge): bf5fc158...
echo <sha-forjado> > .git/refs/replace/<sha-do-forge>     (escrita de arquivo, sem git)

git show <sha>:CHANGELOG.md                  ->  CONTEUDO FORJADO PELO ATACANTE
git --no-replace-objects show <sha>:...      ->  CONTEUDO LEGITIMO
```

**O ataque não usa nenhum comando git** — basta escrever um arquivo em `.git/refs/replace/`. O
guard de branch é irrelevante, e confirmei que ele não menciona `git replace` em bloco nenhum.

### Correção

`--no-replace-objects` como primeiro argumento, nos 3 CLIs. Uma linha cada.

### O que eu errei, e a lição

Deduzi a propriedade de endereçamento por conteúdo — que é real — e **presumi que a ferramenta a
preservava**. Não verifiquei o comportamento do `git show` com camadas de indireção. Foi raciocínio
correto sobre o modelo de dados e incorreto sobre a implementação.

**A regra que fica:** garantia criptográfica do formato não é garantia da ferramenta que o lê.
Quando a segurança depende de "o sha determina o conteúdo", é preciso verificar que o **leitor** não
tem camada de substituição — e `git` tem pelo menos duas (`refs/replace/` e `.git/info/grafts`).

### Superfície adjacente, declarada

`.git/info/grafts` é mecanismo análogo, obsoleto, e **não** é coberto por `--no-replace-objects`.
Avaliar no ML de correção.
