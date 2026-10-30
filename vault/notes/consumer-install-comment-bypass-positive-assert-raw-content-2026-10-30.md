---
date: 2026-10-30
domain: gate-design
severity: low
ml: ML-2A (ROADMAP-2026-10-30-go-install-entrega-versao-errada)
---

# Asserção positiva em conteúdo bruto é enganável por comentário YAML

Mesma classe da nota `grep-normalization-check-defeatable-by-comment-2026-09-27`;
instância nova em `consumer_install_test.go` e `check_consumer_no_latest`.

## Dois sítios afetados

**Go (`internal/generators/consumer_install_test.go`,
`TestConsumerInstall_VersionPinEqualsVersionDotVersion`):**
```go
if !strings.Contains(trigger.content, wantEnv) { ... }
if !strings.Contains(trigger.content, wantURL) { ... }
```
`trigger.content` é o template bruto. As mesmas strings em linhas de comentário YAML
(`#   TRACKFW_VERSION: "11.2.0"` etc.) satisfazem `strings.Contains` — o template pode
não ter bloco `env:` real e o teste passa.

**Bash (`scripts/check-ci-workflow-pin-parity.sh`, `check_consumer_no_latest`):**
```bash
if ! grep -qF 'releases/download/v${TRACKFW_VERSION}/install.sh' "$file"; then
```
Busca no arquivo bruto. Uma linha de comentário contendo a URL satisfaz o grep.

## Medido (2026-10-30)

Template com env block e step de install movidos para YAML comments → todos os 17
check `pin-parity/*/consumer-install-pin` → OK. Script falhou apenas pelo assert_check_fails
do vetor (5) com mensagem divergente — não pelo check positivo direto. Todos os 6 testes
de `TestConsumerInstall_*` passaram.

## Correção

- Go: substituir `strings.Contains(trigger.content, wantEnv)` por
  `strings.Contains(nonCommentLines(trigger.content), wantEnv)` (função já existe no arquivo).
- Bash: substituir `grep -qF '...' "$file"` por `echo "$stripped" | grep -qF '...'`
  (`$stripped` já existe na função, strips comment lines).
- `check_version_pin`: mesmo padrão — rodar contra `$stripped`, não `$file`.

## Por que o vetor indireto não é suficiente

O `assert_check_fails` do vetor (5) reprova com mensagem errada quando o URL está em
comentário (porque o sed muda o comentário, e `check_consumer_no_latest` falha com
"URL pinada ... ausente" em vez de "releases/latest presente"). Um mantenedor que corrija
apenas a mensagem esperada no `assert_check_fails` torna o script verde enquanto o
template permanece sem instalar.
