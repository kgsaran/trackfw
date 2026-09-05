# Nem todo `t.Skip("execute bit not applicable on Windows")` mede a mesma coisa — um era cópia-colada morta

**Contexto:** ML-1A de `ROADMAP-2026-09-05-fechar-os-tres-defeitos-mecanicos-dos-issues-do-consumidor-externo.md`
(issue #279), convertendo os 9 `t.Skip` residuais de classe "plataforma" para o padrão de
`execBitRepresentavelPara`/`execBitNaoExercitado` (ML-4A, `internal/generators/execbit_probe_test.go`).

**O que parecia óbvio, e não era:** os 4 sítios de `scaffold_doctor_test.go` (todos
`if runtime.GOOS == "windows" { t.Skip("execute bit not applicable on Windows (AC5)") }`) pareciam
o mesmo caso, candidatos uniformes ao mesmo probe. Só ao ler `scaffold_doctor.go` (não só o teste)
apareceram duas diferenças reais:

1. **Três dos quatro** (`TestWrongModeDetection_ValidateScript`, `TestWrongModeDetection_StaticScript`,
   `TestExecBitPresent_UmaskNarrowedMode_Accepted`) chamam `checkScaffoldArtifact`/
   `checkValidateScriptArtifact`, que checam o bit de execução **depois** de aprovar o conteúdo —
   `if CurrentGOOS != "windows" && !execBitPresent(path)`. O produto **também** suprime a checagem
   por `CurrentGOOS`, uma decisão já aceita (AC5, coberta por `TestWindowsPlatformGuard`). Isso
   coincide na prática com "bit não representável em NTFS", mas não é a mesma coisa: um host WSL
   escrevendo num bind-mount NTFS teria `CurrentGOOS=="linux"` (produto NÃO suprime) e o bit
   continuaria irrepresentável — só o probe (medir o FS, não o GOOS) protege esse caso, exatamente
   o motivo de existir `execBitRepresentavelPara` em vez de outro `if runtime.GOOS`.

2. **O quarto** (`TestWrongModeDetection_ContentDivergence_TakesPrecedence`) testa que conteúdo
   divergente produz `DoctorScaffoldDivergent` mesmo com modo também errado — e
   `checkScaffoldArtifact` faz `if !bytes.Equal(actual, expected) { return Divergent }`
   **antes** de qualquer checagem de bit (`scaffold_doctor.go:373`, prioridade sobre `:385`). O
   assert nunca toca o bit de execução, em NENHUMA plataforma. O skip ali era cópia-colada das três
   vizinhas — não uma dependência real. Confirmado por mutação: comentar o `bytes.Equal` cedo faz o
   teste reprovar (produto muda de comportamento, teste reage); revertido, passa. Não era vácuo —
   era simplesmente supressão sem motivo, sobrevivendo porque ninguém tinha lido a ORDEM das
   checagens em produção antes.

**A lição generalizável:** uma mensagem de skip idêntica em 4 sítios vizinhos não implica o mesmo
mecanismo por trás. O discriminante é **o que o assert de fato mede**, lido na produção linha a
linha — não a mensagem do skip nem a proximidade textual no arquivo de teste.

**Onde reaparece:** qualquer arquivo de teste com múltiplos `t.Skip`/guard idênticos em sequência
(`internal/generators/roadmap_test.go`, `internal/commands/branch_prune_test.go` têm o mesmo padrão
de repetição — não auditados individualmente nesta sessão, mas candidatos à mesma armadilha).

**Segunda armadilha, distinta:** para os testes onde a garantia sob teste é a PRÓPRIA
primitiva privilegiada (symlink; escrita bloqueada por chmod 0500) — não uma propriedade de um
arquivo já construído — não existe "resto do teste" independente para rodar. Nesses casos
(`update_test.go:35`, `manager_test.go:217`, `manager_persistence_order_test.go:121,201`,
`provenance_test.go:154`) o idioma certo continua sendo `t.Skip` (status explícito, visível com
`-v`) e não o `os.Stderr` + `return` silencioso do `execBitNaoExercitado` — trocar SKIP por um PASS
que não verificou nada é uma REGRESSÃO de transparência, não uma correção. A diferença que importa
é: pode a checagem sob teste ser MEDIDA (não presumida) e a asserção real ainda rodar quando a
condição favorece? Se sim, converta para probe+Stderr. Se a asserção inteira depende da primitiva
privilegiada existir, mantenha `t.Skip`, mas baseado em CONDIÇÃO medida (tentar a operação real e
classificar o erro), nunca em `runtime.GOOS` isolado — mesmo idioma já estabelecido em
`internal/generators/update_test.go:symlinkOrSkip` desde o #221 (anterior ao ML-4A).

**Achado colateral, não corrigido (fora de escopo):** `internal/commands/barrier_contract_test.go:8-10`
tem um comentário de cabeçalho dizendo "cada teste chama t.Skip(...) como primeira linha" — mas o
arquivo hoje tem zero `t.Skip` reais (9 funções `Test*`, todas executando). O ML-2A citado no
próprio comentário já rodou e removeu os skips; o texto que descreve o mecanismo nunca foi
atualizado. Não é um `skip` real, é dívida de documentação — quem tocar esse arquivo deveria
atualizar o cabeçalho.

**Segundo achado colateral, mesma classe:** `internal/generators/execbit_probe_test.go:81-95`
(o próprio arquivo que define `execBitNaoExercitado`) documenta "🔴 LIMITE MEDIDO, e ele não está
fechado no Go... o job de Windows roda `go test` ... SEM `-v`... o fechamento é de UMA palavra e
mora fora de um arquivo de teste (`-v` na linha 384 do workflow), então ficou como lacuna
reportada". **Já foi fechado, no mesmo commit que escreveu esse comentário**:
`.github/workflows/quality.yml:390` roda `go test -v -p 1 -parallel 1 -timeout 20m ./...`, e
`git log -S"run: go test -v -p 1"` aponta para `e6f0d83` (#269) — o MESMO commit de
`git log -1 -- execbit_probe_test.go`. Alguém decidiu fechar a lacuna dentro da própria PR que a
declarou aberta, e o comentário nunca foi atualizado para refletir isso. Não corrigi (arquivo fora
do escopo de escrita desta ML), mas os dois novos probes deste ML (`permissionEnforcementNaoExercitado`
em `manager_persistence_order_test.go`/`provenance_test.go`) já citam o estado correto, não a
lacuna.

**Relatório completo da varredura do acervo (3 runtimes):**
`docs/portabilidade/2026-09-05-varredura-de-skips-e-guards-de-plataforma-no-acervo.md`.
