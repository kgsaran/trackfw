---
status: done
date: 2026-08-28
author: "Zeus (Arquiteto)"
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-28-doctor-compara-o-bit-de-execucao-dos-artefatos-de-scaffold.md"
---

# REQ: modo de execução perdido no validate script, e o `doctor` não compara o bit

> Date: 2026-08-28 | Status: done
| Linear Issue:
| Jira Issue:

## Motivation

**Encontrado ao preparar a release, medido em 2026-08-28.** O `#211` alterou o modo do arquivo:

```
git show 1fc7610 -- scripts/trackfw-validate.sh
  old mode 100755
  new mode 100644          <- perdeu o bit de execucao

.husky/pre-commit  ->  scripts/trackfw-validate.sh
$ ./scripts/trackfw-validate.sh  ->  exit 126 (permission denied)
```

Os outros quatro scripts seguem `100755`; só este perdeu. O gerador escreve com `0755`
(`internal/generators/scaffold.go:739`) — é regressão do **arquivo versionado**, não do gerador.

**Consequência: o `pre-commit` deste repositório parou de rodar o gate de governança.** É a classe que
esta série vem fechando o tempo todo — **controle que parece ativo e não está**.

### O achado que importa mais que o arquivo

**O `scaffold doctor`, entregue no `#211`, compara CONTEÚDO e não MODO.** Ele reportou
`no mismatches` com o script inexecutável. Um artefato pode estar **byte-idêntico** ao template e
ainda assim estar **inerte** — e a ferramenta criada para detectar artefato defasado não vê.

## Acceptance Criteria

- [ ] **AC1** — `scripts/trackfw-validate.sh` volta a `100755` no repositório.
- [ ] **AC2** — O `scaffold doctor` compara o **bit de execução** dos artefatos que o gerador escreve
      como executáveis, nos **3 CLIs**.
- [ ] **AC3** — Artefato com conteúdo correto e **sem** bit de execução é reportado — com estado
      próprio ou mensagem que distinga de divergência de conteúdo.
- [ ] **AC4** — 🔴 **Zero falso-positivo:** artefato que o gerador escreve **sem** bit de execução
      (arquivos de regra, JSON de hook, workflow de CI) **não** pode ser acusado por não tê-lo.
- [ ] **AC5** — **Windows/filesystem sem bit de execução:** a comparação não pode acusar em
      plataforma onde o modo não é representável. Decidir e **declarar**.
- [ ] **AC6** — Gate cobrindo AC2–AC5, com guard de vacuidade, nos 3 runtimes.
- [ ] **AC7** — Falsificação em duas direções: (a) bit removido que deixa de ser reportado; (b)
      artefato não-executável acusado indevidamente.
- [ ] **AC8** — `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` exit 0, com **exit code medido**.

### Acrescentado pela Wave 0 — e muda o escopo

- [ ] **AC9** — 🔴 **O remédio impresso pelo `doctor` é INERTE em Go e Node.** Medido pela Wave 0 e
      **reproduzido por mim**: `os.WriteFile` / `fs.writeFileSync` aplicam `perm` **apenas** no evento
      `O_CREATE`; em arquivo **existente**, o `O_TRUNC` reescreve o conteúdo e **não toca o modo**.

      ```
      Go   os.WriteFile(existente_0644, conteudo, 0755)  ->  permanece 0644   [reproduzido]
      Node fs.writeFileSync(existente_0644, ..., {mode}) ->  permanece 0644
      Py   open(...,'w') + os.chmod(0o755)               ->  vira 0755
      ```

      Sem `os.Chmod`/`fs.chmodSync` **após** a escrita, o `doctor` acusaria, o usuário rodaria
      `trackfw update`, e o bit **continuaria perdido** — em loop. **Corrigir Go e Node é parte desta
      REQ**, não escopo alheio.
- [ ] **AC10** — Comparação por **`mode & 0o100 != 0`**, nunca `== 0755`: umask não-padrão produz
      `0750`/`0700`, que são executáveis e não podem ser acusados.
- [ ] **AC11** — **5 artefatos** requerem o bit (validate script, os 2 attention, os 2 guards); os
      **12** restantes são `0644` nos 3 runtimes e **não** podem ser acusados por não tê-lo.

## Negative scope

- **Não** muda o que o gerador escreve nem os modos que ele usa.
- **Não** adiciona verificação de modo aos artefatos do **manifesto** (catálogo) — é outra superfície,
  com outro mecanismo.
- **Não** trata permissões além do bit de execução (owner, grupo, ACL).

## Linked ADR
ADR: <!-- correcao direta; a decisao de desenho ja esta no ADR-2026-08-27 do scaffold doctor -->

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/done/ROADMAP-2026-08-28-doctor-compara-o-bit-de-execucao-dos-artefatos-de-scaffold.md`
