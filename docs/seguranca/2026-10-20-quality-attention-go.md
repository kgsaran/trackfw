# Revisão de qualidade — atenção Go (ML-1H)

2026-10-20 · Hefesto · REQ-2026-09-05 · worktree `/private/tmp/trackfw-attention-go`.

## Veredito

APROVADO na revisão de qualidade: nenhum bloqueante remanescente no diff final. Liberação da wave ainda depende do `make quality` final em execução pelo Zeus e do red-team. Não equivale a encerramento de AC5–AC7.

## Evidência e avaliação

- `internal/commands/attention.go`: fluxo pequeno, decodificação YAML local sem singleton que encerra processo, JSON tipado, máximo de 1 MiB e deadline absoluto de 2 s. Cleanup não lê stdin. Texto não é avaliado como código; sanitização/truncamento seguem o corpus válido. A goroutine de leitura pode sobreviver ao timeout dentro do processo; o comando hook encerra o processo e esse limite está explícito no comentário, sem servidor persistente chamando esse caminho.
- `internal/commands/attention_test.go`: comparação semântica com scripts legados congelados em `testdata/attention-legacy/`, checksum dos dois arquivos, timestamps normalizados; controles de Unicode, metacaracteres, limite exato/ultrapassado, YAML inválido/multidocumento, cwd sem config, sentinelas externas e credential preservado. Exercita executável real e stdin aberto, além das funções. Não compara wrapper que delega ao próprio Go como oráculo.
- `internal/generators/agentfiles.go` e `attention_hooks_test.go`: seis emissores, migração por igualdade das formas bare/ancoradas, deduplicação e preservação de campos customizados; testes de idempotência e metadata. Kiro/Copilot permanecem arquivos dedicados integralmente gerados, conforme contrato anterior. Testes existentes dos guards mantêm as asserções, alterando somente referências de atenção.
- Wrappers fonte/cópia continuam cobertos por `TestScripts_LiteralMatchesVersionedCopy`. Teste novo executa as linhas nativas em sh/bash/zsh disponíveis com PATH contendo só o binário e verifica wrappers fail-soft sem binário e com binário que falha. Testes antigos que fixavam sed/Python foram substituídos pelo contrato nativo e cobertura comportamental, sem manter exigência artificial de implementação removida.
- `scripts/check-output-encoding-declared.sh`: ALVO 1 mantido; ALVO 2 agora exige os dois wrappers nativos e recusa Python/jq. `check-crlf-normalize-capture.sh` mantém piso 52. Nenhuma exceção de contenção foi acrescida ao gate.
- P21 e workflow: instrumento observacional sempre exit0, processos próprios com timeout/cleanup, streams drenados simultaneamente, shells absolutos e PATH do filho só com binário. Mede API sinal/cleanup e preservação de credential; declara explicitamente que não executa eventos de agente. Sem alegação de aceite de schema por fornecedor ou AC5–AC7.

## Integração detectada pelo gate amplo

`make quality` executado pelo Zeus falhou em `internal/pathguard`, teste `TestNoContainmentEmitterOutsidePathguard`: três chamadas de `RejectSymlinks` em `attention.go` (linhas 96, 161, 168 na versão revisada) não delegavam ao emissor canônico `RejectAndReport`. CORRIGIDO pelo proprietário e auditado nesta revisão: os três sítios usam `RejectAndReport`, o `Beneath` redundante saiu e nenhum teste/allowlist foi alterado. Retornos locais mantêm exit0. `docs/cli-parity.md:8997` distingue o diagnóstico canônico stderr de erros silenciosos de payload/config/I/O. Zeus reexecutou os nomes exatos `TestContainmentAnalyserOverTheLiveTree|TestNoContainmentEmitterOutsidePathguard`: PASS, 0.181 s.

## Validação e limites

Inspeção de diff completo e arquivos novos, Wave 0, adendo ADR de 20/10 e notas do vault sobre cópia/literal, rootDir e PATH. `trackfw context` executado (exit0, avisos legados/globais). Suites focadas, build e vet verdes comunicados pelo orquestrador/proprietário; não foram duplicados nesta revisão. O primeiro `make quality` reprovou pelo achado acima, já corrigido. Reexecução global está em andamento (sessão 10608 do Zeus); esta revisão não afirma gate amplo verde. Não houve alteração de produto, guard/validator ou trabalho do PR externo #590 nesta revisão.
