---
status: Proposed
date: 2026-10-16
author: "zeus-tf"
---

# ADR: no Windows, o `validate` avisa quando o projeto concede escrita a outro usuário

> Date: 2026-10-16 | Status: Proposed

REQ: `docs/req/REQ-2026-09-03-as-217-falhas-reais-de-windows-colapsam-em-poucas-causas-e-tres-delas-exigem-decisao-antes-de-codigo.md`
Substitui: `ADR-2026-10-16-dacl-explicita-para-arquivos-de-governanca-no-windows.md` (Rejected)

## Context

No Windows, o `0o600`/`0o700` que o trackfw pede em 5 sítios (`identity.json`, manifesto de
integrações, artefato renderizado, quarentena e proveniência de terceiros) é inerte: o Go só alterna
o atributo somente-leitura. A ACL real vem da herança do diretório pai.

Medido no CI x64:

- `%USERPROFILE%\.trackfw\` — só dono, SYSTEM e Administrators (run `38902064559`).
- Projeto sob a raiz `C:\` — `NT AUTHORITY\Authenticated Users:(I)(M)` herdado (mesmo run).
- Com usuário local **não-admin** real (run `38918006930`): com DACL explícita no arquivo, ele não
  sobrescreve nem apaga a proveniência — mas **renomeia a raiz do projeto e a recria** com conteúdo
  forjado, sempre que nenhum processo da vítima tem a pasta aberta.

Com Modify alheio na raiz, o projeto inteiro (código, hooks, `trackfw.yaml`, `.trackfw\`) está nas
mãos do co-inquilino. Nenhuma ACL que o trackfw ponha num arquivo muda isso.

## Decision

**D1 — O trackfw não aplica DACL própria.** Os sítios continuam pedindo `0o600`, que vale no POSIX.
No Windows, a proteção é a do diretório onde o usuário pôs o projeto — **limite declarado**.

**D2 — O `validate` diz a verdade.** No Windows, uma regra **informativa, não bloqueante** lê a DACL
da raiz do projeto e do `.trackfw\` (se existir) e avisa quando alguma ACE *allow* que se aplica ao
próprio objeto, ou que é herdável para arquivos (`OI`), concede escrita, exclusão ou troca de DACL
(`FILE_WRITE_DATA`, `FILE_APPEND_DATA`, `FILE_ADD_FILE`, `FILE_ADD_SUBDIRECTORY`, `DELETE`,
`FILE_DELETE_CHILD`, `WRITE_DAC`, `WRITE_OWNER`, `GENERIC_WRITE`, `GENERIC_ALL`) a um SID fora do
conjunto: usuário atual, `SYSTEM` (`S-1-5-18`), `Administrators` (`S-1-5-32-544`),
`CREATOR OWNER` (`S-1-3-0`), `OWNER RIGHTS` (`S-1-3-4`). O aviso nomeia o caminho, os SIDs (com
nome resolvido quando possível) e o remédio: mover o projeto para dentro do perfil do usuário ou
remover a herança.

**D3 — Erro ao ler a DACL vira aviso**, nunca silêncio (mesmo princípio do A7 do
ADR-2026-10-14). Fora do Windows a regra não existe.

**D4 — O legado (ML-W2c) fica absorvido**: o aviso cobre projeto e `.trackfw\` existentes, sem
depender de quando os arquivos foram gravados.

## Consequences

- Nenhum código que muda ACL de arquivo do usuário. O trackfw só lê.
- Num projeto sob `C:\` o usuário passa a ser avisado; antes, nada.
- O teste `TestSave_WritesAtomicallyWithPermissions` passa a afirmar no Windows o que é medido (o
  modo não é `0o600`), e a afirmação POSIX fica nos demais SOs.
- Resíduo declarado: o trackfw continua gravando sob a herança do diretório; o aviso é a defesa.

## Alternatives Considered

- **DACL explícita por arquivo** (`internal/fsperm`): implementada, medida e rejeitada — barra só a
  sobrescrita direta; a troca da raiz contorna; trazia dano novo (R-B, junction) e risco de dono
  implícito (R-A).
- **Bloquear o `validate`**: o projeto é do usuário e o ambiente pode ser deliberado (máquina
  pessoal). Rejeitada; aviso informativo.
