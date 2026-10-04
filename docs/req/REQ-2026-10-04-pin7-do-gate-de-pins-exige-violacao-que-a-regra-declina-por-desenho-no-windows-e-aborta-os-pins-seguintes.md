---
status: Done
date: 2026-10-04
author: "zeus-tf"
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-10-04-pin7-do-gate-de-pins-exige-violacao-que-a-regra-declina-por-desenho-no-windows-e-aborta-os-pins-seguintes.md"
---

# REQ: pin7 do gate de pins exige violacao que a regra declina por desenho no Windows e aborta os pins seguintes

> Date: 2026-10-04 | Status: Done
| GitHub Issue: #421

## Motivation

O `scripts/check-validate-rule-pins.sh` espera, no `pin7-noexec`, que a regra
`credential_guard_hook_resolvable` acuse um hook "present but not executable". No Windows a regra
**declina essa checagem por desenho**: `internal/validator/validator_credential_guard.go:493`
(`CurrentGOOS != "windows" && info.Mode()&0111 == 0`), com a garantia escrita em
`internal/validator/goos.go`. Lá, o Go reporta `-rw-rw-rw-` para todo arquivo regular, inclusive um
`.exe`.

O pin acusa "fixture broken or rule regressed", mas o estado real é o terceiro, que a mensagem não
oferece: **a regra está corretamente guardada**.

**Reproduzido pelo arquiteto na VM em 2026-10-04:**
- ambiente: `MINGW64_NT-10.0-26200-ARM64`, `/tmp` em `ntfs (binary,noacl,posix=0,usertemp)`, `main` em
  `ca3342e`;
- resultado: o `pin6` passa, e o `pin7` falha com `vacuity … none found (rc=0)`.

🔴 O `raise SystemExit` do `pin7` aborta o laço, então **os pins 8 a 20 nunca rodam no Windows**. A falha
esconde o resto do gate.

Medições do consumidor externo na #421:
- o mount é o mesmo no `windows-latest` (run 36068087897) e em x64 nativo;
- o caminho do `icacls` está falsificado: o MSYS não consulta a ACL;
- o discriminante `[ -x ]` do bash mede outra coisa que a regra Go;
- o `os.Stat` do Go dá `0666` até para o `.exe`.

Os testes Go **já** fixam os dois lados da regra pelo seam `CurrentGOOS`
(`validator_credential_guard_test.go:60` força `linux`, `:968` força `windows`). A lacuna é só do gate
de shell.

### Decisão
O mesmo seam que a regra usa decide o pin. Quando o binário sob teste é Windows, o `pin7` **afirma o
comportamento guardado**: nenhuma violação desta regra para a fixture `noexec`. Ele imprime um `OK`
próprio (`pin7-noexec-windows-guarded`), que nomeia a garantia (`internal/validator/goos.go`). Não é
`SKIP`: a afirmação é verificável lá, e é a oposta. Fora do Windows, nada muda.

O discriminante de plataforma tem de responder pelo **binário Go** (o processo que avalia a regra), e não
pelo `[ -x ]` do bash.

## Acceptance Criteria

- [x] **AC1** — 🔴 **Wave 0:** threat model curto, com parecer em `docs/seguranca/`. O discriminante de
  plataforma pode errar e silenciar o pin7 numa plataforma POSIX? O pin "guardado" pode ficar vacuoso,
  por exemplo com a fixture ausente?
      ✅ Evidência: `docs/seguranca/2026-10-04-wave0-pin7-windows-guardado.md`: discriminante `go version -m` (conferido pelo arquiteto: nativo `.exe` → darwin, cruzado sem `.exe` → windows) e anti-vacuidade
- [x] **AC2** — Na VM Windows, `check-validate-rule-pins.sh` sai com rc=0, e o log mostra
  `OK [validate-rule-pins/pin7-noexec-windows-guarded]` **e** os pins 8 a 20 executados. Se algum pin de
  8 a 20 falhar no Windows, ele entra nesta REQ (Regra Dura) com a causa medida.
      ✅ Evidência: VM Windows ARM64, `/tmp` em NTFS `noacl`, medido pelo arquiteto: rc=0, 32 OK, `pin7-noexec-windows-guarded`, pins 8–20 executados e verdes
- [x] **AC3** — No macOS/Linux, a saída do gate fica idêntica à de hoje para o pin7 (`OK
  [validate-rule-pins/pin7-noexec]`).
      ✅ Evidência: macOS, medido pelo arquiteto: rc=0, 32 OK, `OK [validate-rule-pins/pin7-noexec]` igual ao de antes
- [x] **AC4** — Falsificação nas duas direções:
  - forçar o discriminante para "windows" num host POSIX faz o braço guardado reprovar, porque a regra
    acusa a violação;
  - forçar "não windows" na VM faz o braço POSIX reprovar, como hoje.
      ✅ Evidência: braço guardado forçado no macOS → rc=1 (a regra acusa); braço POSIX forçado na VM → rc=1
- [x] **AC5** — A mensagem de vacuidade do pin deixa de oferecer só a dicotomia "fixture broken or rule
  regressed" quando a plataforma guardada for possível; ela nomeia o terceiro estado.
      ✅ Evidência: mensagem de vacuidade: "fixture broken, rule regressed, or the rule is guarded on this platform — see the platform-guarded pins"
- [x] **AC6** — `make parity-rest` EXIT=0 no macOS; CI verde.
      ✅ Evidência: `make parity-rest` EXIT=0; CI do PR #522: 20/20

## Negative scope

- Mudar a regra Go ou o `goos.go`.
- Montar `/tmp` com ACL, ou construir a fixture por `icacls` (falsificado na #421).
- Outros gates que não o `check-validate-rule-pins.sh`. A varredura por "not executable" em `scripts/`
  achou só este.

## Linked ADR
<!-- governado pela garantia já escrita em internal/validator/goos.go -->

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/done/ROADMAP-2026-10-04-pin7-do-gate-de-pins-exige-violacao-que-a-regra-declina-por-desenho-no-windows-e-aborta-os-pins-seguintes.md
