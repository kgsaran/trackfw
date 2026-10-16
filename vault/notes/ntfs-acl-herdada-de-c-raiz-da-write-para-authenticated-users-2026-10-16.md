# NTFS: arquivos sob `C:\` raiz herdam Modify para Authenticated Users

> ML-W2a · REQ-2026-09-03 · medido em run `38902064559` (x64, 2026-10-16)

## Fato

No runner x64 do GitHub Actions (e na configuração padrão do Windows), a raiz `C:\` tem:

```
NT AUTHORITY\Authenticated Users:(OI)(CI)(IO)(M)
```

`(OI)(CI)(IO)(M)` = ObjectInherit + ContainerInherit + InheritOnly + Modify.

Efeito: **todo arquivo e diretório criado sob `C:\`** (direta ou indiretamente) herda uma ACE de
Modify para `NT AUTHORITY\Authenticated Users`. Esta ACE permite Write e Delete.

O `C:\Users\<user>` tem ACL própria que **bloqueia** essa herança para o perfil do usuário —
mas apenas para a subárvore de perfis. Um diretório criado diretamente em `C:\` (como
`C:\meuprojeto`) herda o Modify.

## Relevância para o trackfw

Os 5 sítios com `0o600` usam `atomicWrite`:
1. `os.MkdirAll(dir, 0o700)` → cria o diretório com herança do pai (0o700 é inerte no Windows)
2. `os.CreateTemp(dir, ...)` → cria o arquivo com herança do diretório
3. `Chmod(0o600)` → só alterna o bit READONLY; **não modifica a ACL**
4. `os.Rename(tmp, final)` → não modifica a ACL

Resultado: ACL do arquivo = ACL herdada do diretório pai. Se o projeto ficar sob `C:\`, os
arquivos `.trackfw\identity.json`, `.trackfw\integrations-manifest.json`,
`.trackfw\thirdparty-provenance.json` e `.trackfw\thirdparty-quarantine\<hash>.json` terão
`Authenticated Users:(I)(M)`.

## O que NÃO acontece com projetos em `C:\Users\<user>\`

`C:\Users\<user>` tem ACL própria com apenas SYSTEM, Administrators e o dono. Arquivos criados
ali **não** herdam o Modify de Authenticated Users. Resultado confirmado:

```
C:\Users\runneradmin\.trackfw\identity.json NT AUTHORITY\SYSTEM:(F)
                                            BUILTIN\Administrators:(F)
                                            runnervm0xl3a\runneradmin:(F)
```

## Implicação para a decisão ML-W2b

- Projeto em `~/` (home do usuário): sem escrita alheia → declarar o limite pode ser suficiente.
- Projeto sob `C:\` raiz (ou outro diretório público): escrita alheia presente → DACL explícita
  necessária para proteger a proveniência de terceiros.

## Referência

`docs/portabilidade/2026-10-16-acl-0600-ntfs-ci.md` — tabela completa + icacls bruto.
