---
name: req-new-symlink-write-e-dedup-lexical-case
description: 2026-09-03, ML-1A do resolvedor de REQ — req new escreve através de symlink em req_dir/<agente>; e a dedup lexical duplica em FS case-insensitive. APROVA COM RESSALVAS, ambos sem correção.
metadata:
  type: project
---

Barreira do `fix/resolvedor-de-req-cobre-o-layout-canonico-…` (ML-1A `f7963c7`, ML-2A `31bcdef`).
Parecer: `docs/seguranca/2026-09-03-parecer-resolvedor-de-req.md`. Veredito **APROVA COM RESSALVAS**;
nada corrigido no PR.

**Achado 1 — escrita fora do projeto, NOVO no ML-1A.** `req new` em `roadmap_namespacing: by_agent`
passou a gravar em `req_dir/<agente>/`. Um **symlink de diretório plantado em `req_dir/default`**
(constante usada quando `agents:` é vazio) redireciona a escrita para fora, nos 3 CLIs — e
**sobrescreve** arquivo existente sem aviso. `MkdirAll`/`mkdirSync`/`makedirs` sobre symlink-para-dir
retornam sucesso e não são guarda.

**Why:** não bloqueei porque o **mesmo furo já existe em `origin/main` no `roadmap new`**
(`roadmap_dir/default`) — medido com `git archive origin/main`. É extensão de classe, não classe
nova; bloquear trocaria dois furos por um.

**How to apply:** ao revisar qualquer escritor deste repo, a pergunta é "algum componente do caminho
vem do disco/config sem `lstat`?". O leitor (`resolveAgentNamespaces`) recusa symlink há tempos
(AC12/AC13); o **escritor não recusa nada** — assimetria escritor/leitor no eixo de segurança.

**Achado 2 — dedup lexical duplica em FS case-insensitive, NOVO no ML-1A.** `req_dir/Backlog` em
APFS/NTFS é o mesmo dir que o `backlog` hardcoded, mas string diferente após
`Clean`/`normalize`/`normpath`: cada REQ conta 2x e cada violação sai em duplicata (4 onde a AC prevê
2, nos 3 CLIs). **Verde no CI Linux, vermelho na máquina do dev.** Identidade de filesystem
discrimina em APFS (medido: `ino/dev` iguais, `os.SameFile: true`), mas `st_ino`/`ino` em **NTFS
não foi medido** — não recomendar como contrato sem a sonda de Windows.

Relacionado: [[project_update_discover_symlink_follow_arbitrary_write]],
`vault/notes/lstat-nao-ve-junction-e-guarda-de-folha-nao-olha-ancestral-2026-08-31.md`.
