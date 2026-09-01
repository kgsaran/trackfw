# `CurrentGOOS`/`_platform`/`_current_platform` refletem o SO do binário, não do host — WSL continua coberto

**Contexto:** ao portar o defeito 3 da issue #216 (bit de execução NTFS,
`ROADMAP-2026-08-31-portar-as-correcoes-do-reporter-da-issue-216`, ML-1A), a checagem "script do
guard não é executável" (`internal/validator/validator_credential_guard.go`,
`validator_git_branch_guard.go`, e equivalentes Node/Python) ganhou um guard de plataforma:

```go
case CurrentGOOS != "windows" && info.Mode()&0111 == 0:
```

Isto suprime a checagem inteira quando `CurrentGOOS == "windows"`. Lido isoladamente, parece
enfraquecer um controle de segurança (credential guard/git branch guard resolvability) em todo
ambiente Windows.

**Por que não enfraquece:** `CurrentGOOS` é seedado de `runtime.GOOS` (Go) — e os equivalentes
`_platform`/`_current_platform` de `process.platform`/`sys.platform` — que refletem o SO para o qual
o **binário foi compilado/o interpretador roda**, não o SO físico do host. Um binário Linux do
`trackfw` rodando dentro de WSL (kernel Linux sobre Windows, filesystem ext4 via VHD, onde o bit de
execução **é** representável e **é** preservado por `chmod`) reporta `"linux"`, não `"windows"` — a
checagem continua ativa exatamente onde o bit continua sendo um sinal real. O guard só desarma
quando o binário é genuinamente Windows-nativo (NTFS/FAT), onde `Mode()&0111` é **sempre** `0`
mesmo imediatamente após `chmod(0o755)` — ali a checagem nunca foi um discriminante, era
sempre-verdadeiro travestido de achado.

**Como verificar rápido, se a dúvida reaparecer:** reverter o guard localmente e rodar o teste que
afirma "não dispara no Windows" — ele falha, provando que o guard é a única coisa impedindo o
falso-positivo:

```bash
sed -i.bak 's/case CurrentGOOS != "windows" && info.Mode()&0111 == 0:/case info.Mode()\&0111 == 0:/' \
  internal/validator/validator_credential_guard.go
go test ./internal/validator/... -run TestCredentialGuardHookResolvable_WindowsNaoDisparaBitDeExecucao -v
# restaurar: mv validator_credential_guard.go.bak validator_credential_guard.go (ou git checkout)
```

**Onde reaparece:** mesmo padrão já usado em `internal/generators/scaffold_doctor.go` (Cenário 179
de `check-gates-falsify.sh`, `REQ-2026-08-28`) — este é o segundo uso do mesmo idioma, não o
primeiro. Se aparecer um terceiro guard de plataforma equivalente, checar (a) se a condição
suprimida é de fato sempre-verdadeira na plataforma suprimida — não presumir — e (b) se o `sed` de
falsificação em `check-gates-falsify.sh` mira o substring da condição, não a cláusula `case`
inteira (ver
[[falsify-cenario-pina-linha-de-fonte-por-sed-guard-de-plataforma-quebra-2026-08-31]]).

**Achado da barreira:** `docs/seguranca/2026-09-01-barreira-do-port-do-reporter-da-issue-216.md`,
Ponto 5 — APROVA, nenhum enfraquecimento líquido.
