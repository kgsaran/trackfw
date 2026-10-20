# Wave 0 — atenção nativa Go (2026-10-20)

## 1. Enumeration completeness — ✅ concluída

Veredito: libera implementação condicionada aos controles abaixo; não fecha AC5–AC7. REQ-2026-09-05 e roadmap pareado estão em `docs/roadmaps/wip/`. `trackfw context` e `trackfw validate` retornaram 0 nesta árvore; há avisos legados/globais, portanto exit 0 não significa ausência de warnings. Nenhuma linha de produto foi escrita nesta análise.

Busca por `attentionSignalScript`, `trackfw-attention-signal.sh`, `trackfw-attention-cleanup.sh` e `GenerateAttentionScripts` fecha estas superfícies:

| Superfície | Evidência e obrigação |
|---|---|
| Comando/runtime | Novo `internal/commands/attention.go` e testes; registrar em `internal/commands/root.go:54–85`. Comandos exatamente `trackfw attention signal` e `trackfw attention cleanup`. |
| Seis emissores | `internal/generators/agentfiles.go`: Claude 297/302/405/410, Codex 454–455/634/640, Gemini 465–466/805/813, Kiro 984/991, Copilot 1138/1146, Cursor 1363–1366. Copilot usa campo `bash`; respeitar schema do fornecedor sem inventar campo genérico. |
| Migrações | `migrateHookCommand` 1492 compara matcher e comando por igualdade; migrar formas relativas e ancoradas previamente emitidas antes de deduplicar. Cursor tem arrays planos e legado top-level; Kiro/Copilot arquivos dedicados, contrato de propriedade distinto dos arquivos compartilhados. Preservar hooks personalizados. |
| Templates e cópias | `internal/generators/scaffold.go:1099–1164` e `scripts/trackfw-attention-{signal,cleanup}.sh`: virar wrappers. `GenerateAttentionScripts` 1169 recebe rootDir; preservar escrita nessa raiz, não cwd acidental. |
| Fluxos de geração | `internal/generators/scaffold.go:149`, `internal/discover/discover.go:79`, `internal/generators/update.go:87/2718`; `internal/generators/scaffold_doctor.go:279–280` compara templates. Auditar update dry-run e alvos declarados em update.go:2697–2707/2988–2992. |
| Assets/catálogo | `internal/integrations/assets/catalog.json:37–46` enumera paths de hooks dos seis fornecedores; não contém implementação shell de atenção. Inspeção/doctor devem continuar reconhecer artefatos após atualização. |
| Consumidor | `internal/serve/api_attention.go:25–34` lê `.trackfw-attention.json` e JSON inválido vira inactive. Manter tool/message/level/timestamp; level `action_required`, timestamp UTC. |
| Testes/gates | `internal/generators/{agentfiles_test.go,hooks_test.go,update_test.go,scaffold_test.go,scaffold_parity_test.go,scaffold_literal_copy_parity_test.go,copilot_hooks_parity_test.go,scaffold_doctor_test.go,scaffold_leaf_guard_test.go}`; `scripts/check-output-encoding-declared.sh` e `scripts/check-crlf-normalize-capture.sh` possuem pressupostos sobre implementação antiga. Testes de credential/git guard que mencionam atenção são controles de não regressão, não licença para editar guards. |

Antes de substituir scripts, congelar corpus legado real dos dois scripts e checksum em testdata (localização exata escolhida pelo implementador e auditada). Comparar Go contra corpus independente, não contra wrappers que chamam o próprio Go. A nota `vault/notes/copia-versionada-do-attention-signal-esta-obsoleta-e-sem-guarda-2026-09-02.md` documenta a antiga cegueira; seu addendum registra o teste literal/cópia agora existente. Ler também `go-generateattentionscripts-cwd-vs-rootdir-2026-08-04.md`, `codex-hooks-de-projeto-so-rodam-em-projeto-trusted-2026-08-11.md`, `barrier-gate-auto-referencial-vira-fork-bomb-2026-09-30.md` e nota zsh de 20/10 no checkout original.

## 2. Threat model — ✅ concluído

Adversário primário: implementador apressado pode portar apenas o arquivo scripts/ e deixar o literal distribuído intacto; arquiteto otimista pode aceitar igualdade wrapper→Go como prova de paridade ou teste bash como execução real do agente Windows. Todos passam gates superficiais sem entregar o comportamento prometido.

Entrada JSON, YAML e caminhos são não confiáveis. Não executar conteúdo question/command, não interpolar em shell, serializar com JSON nativo; prioridade question > command > texto derivado de tool_name, máximo 300 runas Unicode na mensagem, controles tratados consistentemente com corpus. Limitar bytes de stdin e tempo de leitura; fluxo que nunca fecha não pode travar hook. Cleanup não precisa consumir stdin.

**Bloqueante de desenho S1:** `internal/config/config.go:234–257` usa singleton `Load()` e `osExit(1)` para YAML malformado/multidocumento. Chamar essa API diretamente viola exit 0 em erro de runtime. Usar leitura/decodificação sem saída de processo; testar o executável real com YAML malformado. `root.go:37–40` apenas silencia usage, não resolve esse risco.

**Bloqueante de desenho S2:** proteção textual do shell (`scaffold.go:1126–1128`) não prova contenção física. Resolver raiz e conferir caminho com `internal/pathguard/pathguard.go`: `ResolveRoot` 280, `Beneath` 60, `RejectSymlinks` 103, escrita guardada 144. RejectSymlinks cobre ModeSymlink e ModeIrregular (junction Windows). Recusar também folha e ancestral antes de criar/remover; não seguir link para fora nem converter caminho recusado em escrita externa. Config apenas `trackfw.yaml` do cwd; não procurar projeto ancestral. Cleanup remove somente `.trackfw-attention.json`; `.trackfw-credential-guard.json` permanece byte-idêntico.

Erros de payload/config/I/O retornam 0 (no-op), atenção é informativa. Comandos não recebem prefixo polyglot de guard: nota `codex-guard-polyglot-aborta-zsh-equals-2026-10-20.md` demonstra abort antes de binário sob zsh. Não alterar guard, hooks locais/globais, PR #590 ou instalar modelos. Gate da wave é verificador finito; nunca reentra em barrier.

## 3. Falsification targets in both directions — ✅ concluídos

Cada linha exige regressão negativa e controle legítimo; teste só de exit 0 é insuficiente: verificar bytes, ausência de arquivo externo e tempo decorrido.

| Superfície | Falso negativo que gate deve apanhar | Falso positivo que gate deve recusar |
|---|---|---|
| Runtime/JSON | Remover escaping/truncamento permite JSON inválido ou >300 runas; inserir payload comando deve continuar literal. `attention_test.go` mede schema e conteúdo. | Unicode astral, aspas, barras, pergunta e command legítimos preservados; timestamp variável normalizado só na comparação. |
| Config/cwd | YAML inválido força exit1; busca ancestral cria banner fora do cwd; absolute/traversal/backslash escapam raiz. Teste de subprocesso e filesystem. | YAML CRLF, caminho relativo com espaços e default válido produzem sinal na pasta correta; fora do projeto é no-op0. |
| Confinamento | Mutar guarda para aceitar symlink/junction de pasta ou folha permite escrita/remoção externa; tests pathguard e attention devem falhar com sentinela externa intacta. | Diretório legítimo ainda inexistente pode ser criado; raiz resolvida legítima funciona, cleanup ausente é no-op. |
| Cleanup | Remover guard ou alargar wildcard apaga credential/sentinela; attention tests verificam bytes dedicados. | Cleanup válido remove atenção mesmo com stdin aberto; não precisa aguardar payload. |
| Limites | stdin aberto ou acima de teto fica preso/aloca sem limite; teste com pipe vivo exige teto de duração e sem escrita parcial. | Payload válido abaixo do limite e stdin fechado chega inteiro; processo sai0. |
| Seis emissores | Deixar um CLI no script legado ou aplicar polyglot; testes estruturais por fornecedor falham, incluindo campo bash Copilot e arrays Cursor. | Hooks customizados, matchers, timeout e entradas de guard permanecem; duas atualizações não duplicam. |
| Migrações | Migrar só forma bare deixa forma ancorada duplicada; fixtures por forma legada e fornecedor. | Comando customizado com substring parecida não deve ser reescrito; igualdade reconhecida, não replace amplo. |
| Templates/cópias/corpus | Alterar só literal ou cópia deve falhar `scaffold_literal_copy_parity_test.go`; corpus checksum impede trocar referência por wrapper. | Wrappers novos corretos não devem ser rejeitados por exigir Python/jq de implementação retirada. Atualizar gates sem desligar controles de encoding restantes. |
| Discover/update/doctor | Geração em cwd em vez de rootDir ou doctor prescrevendo corpo antigo falha testes existentes de discover, update e scaffold doctor. | rootDir distinto de cwd, dry-run e hooks personalizados preservados. |

Gates de entrega: `go build ./...`; `go test ./internal/commands ./internal/generators ./internal/integrations`; `go vet ./...`; `make quality`; `trackfw validate --json`. Acrescentar testes de atenção e falsificações acima à suite executada; não substituir verificação comportamental por grep ou relatório não vazio. Wave 0 libera escrita somente depois da auditoria Zeus; red-team mede implementação final antes do encerramento ML.

## 4. Declared residual — ✅ concluído

Este relatório não afirma execução nativa Windows pelos seis agentes. Um `powershell -Command`, Git Bash ou teste Go não comprova evento real, schema aceito, cwd, confiança, PATH e shell efetivamente selecionado pelo agente. AC5–AC7 permanecem pendentes até evidência Windows em runtime real, com agente/versão/evento/OS registrados. O Codex trusted documentado no vault é condição adicional; não editar configuração real do usuário para obter verde.

Não se promete proteção contra adversário concorrente substituindo diretórios entre check e operação (TOCTOU): utilitários existentes são base de contenção, não handles filesystem à prova de corrida. Atenção é arquivo informativo e pode conter texto sensível vindo do próprio payload; não enviar rede nem imprimir payload em logs. Não resolve disputa entre sessões concorrentes (cleanup pode retirar sinal de outra sessão), identidade/autorização de autores, indisponibilidade do binário/PATH, ou cancelamento pelo processo chamador. Não portar correções de guard ou PR externo. Paridade com shell aplica aos casos válidos congelados; erros legados divergentes jq/Python não devem prevalecer sobre contrato explícito no-op0/limites/contenção. Valores exatos do teto de bytes e timeout precisam ficar definidos e testados na implementação; aprovação final não aceita “limitado” sem números.
