# `os.Chmod` segue symlinks — janela WriteFile+Chmod medida

> 2026-10-31 · hades-tf · red team ML-2A (REQ-2026-08-30, débitos de higiene)

## O fato não óbvio

`os.Chmod` em Go segue symlinks: aplica o modo no **inode do alvo**, não no link.

Consequência: num padrão `os.WriteFile(dest, …) + os.Chmod(dest, 0755)`, um atacante local que
plante um symlink em `dest` **entre** as duas chamadas recebe a promoção de modo no arquivo-alvo.

## Medição (função real `writeValidateScript` via `InstallGates`, 5.000 iterações)

Medido via `TestZZ_WriteValidateScript_SymlinkRace` em `internal/discover/` (teste removido após medição):

```
iterations=5000
WriteFile-through-symlink(TOCTOU) = 1912  (38%; atribuível ao WriteFile de InstallGates)
Chmod-through-symlink(TOCTOU)     = 171   (3.4%; exclusivamente de InstallGates — racer não faz Chmod)
```

`otherTarget` é removido e recriado em modo 0600 a cada iteração (`resetOther`). O racer faz `Remove(dest)` antes de cada `WriteFile(dest,"racer")` — portanto dest nunca é symlink quando o racer escreve. Os 1912 WriteFile-wins são inteiramente do WriteFile de InstallGates (guard viu arquivo regular, racer plantou symlink no TOCTOU window, WriteFile seguiu o symlink). A contagem de Chmod (171) é atribuível ao `os.Chmod` de `writeValidateScript`.

O guard `RejectSymlinks` captura o symlink quando já está em dest no momento da verificação; o TOCTOU ocorre entre o guard e o WriteFile/Chmod. O Chmod-específico (3.4%) é menor que o WriteFile (38%) porque a janela é ainda mais estreita.

## Onde isso aparece neste repositório

Padrão `WriteFile + Chmod` em três sítios adicionados por `72aac048`:
- `internal/discover/discover.go` — `writeValidateScript` (scripts/trackfw-validate.sh)
- `internal/generators/scaffold.go:~1980` — `generateCommitMsgHook` (.husky/commit-msg)
- `internal/generators/scaffold.go:~2054` — `generateHuskyHook` (.husky/pre-commit)

Todos têm leaf guard (`rejectScaffoldPath`) antes do `WriteFile`, não entre `WriteFile` e `Chmod`.
O guard verifica o **caminho** (string), não o inode — não detecta symlinks.

O padrão pré-existente em `scaffold.go:1077` tinha a mesma janela sem o `Chmod` (ver nota TOCTOU
geral em `toctou-verificar-o-nome-e-usar-o-nome-sao-duas-resolucoes-2026-09-06.md`).

## Hardlink: o `os.Chmod` promove o modo do inode compartilhado

Para hardlinks, o efeito é imediato e sem janela de corrida: `os.WriteFile` grava no inode
compartilhado (sobrescreve conteúdo em todos os nomes do inode), e `os.Chmod` eleva o modo do
mesmo inode. Um hardlink plantado em `dest` antes de `writeValidateScript` redireciona ambas as
operações para o inode-alvo.

## Superfície de ataque

Requer acesso local de escrita ao diretório pai (scripts/ ou .husky/) **durante** `trackfw init`.
Em projetos de usuário único isso é irrelevante. O risco aumenta em projetos compartilhados na
mesma máquina ou em ambientes onde o diretório do projeto está em /tmp.

## Conclusão para futuros implementadores

Sempre que adicionar `os.WriteFile(dest) + os.Chmod(dest, …)`:

1. O padrão tem janela TOCTOU; o `os.Chmod` eleva o inode (via symlink ou hardlink).
2. **Primitiva correta: `pathguard.GuardedWrite(root, filename, data, mode)`**. Ela:
   - Chama `RejectAndReport(root, filename)` como guard.
   - Cria um temp file no mesmo diretório.
   - Chama `temporary.Chmod(mode)` no fd aberto — não re-resolve o nome.
   - Fecha o fd e chama `os.Rename(tmp, filename)`.
   - O rename **substitui** uma folha symlink em `filename` em vez de segui-la.
   - O rename cria novo inode — rompe qualquer hardlink que apontava para `filename`.
   - Fecha R-4 (hardlink) e ambas as janelas TOCTOU (WriteFile 38% e Chmod 3.4%).
3. **`f.Chmod(0755)` no fd fecharia apenas a janela do Chmod (3.4%)**: WriteFile ainda tem a janela maior (38%). Use `GuardedWrite` para fechar os dois.
4. **`O_NOFOLLOW` sozinho não fecha a janela do Chmod:** se usado apenas no `OpenFile`, o `os.Chmod(dest, 0755)` logo após ainda resolve `dest` pelo nome.
5. **`O_NOFOLLOW` não ajuda com hardlinks:** hardlinks não são detectados por `ModeSymlink`.
6. O padrão `WriteFile+Chmod` foi aceito como residual (nível BAIXO) enquanto a migração para `GuardedWrite` não for feita.
