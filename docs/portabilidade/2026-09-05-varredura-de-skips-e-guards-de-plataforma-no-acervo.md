# Varredura de `skip`/guard de plataforma no acervo — ML-1A (#279)

> Roadmap: `ROADMAP-2026-09-05-fechar-os-tres-defeitos-mecanicos-dos-issues-do-consumidor-externo.md`
> Agente: `artemis-tf` · Escopo de escrita: 5 arquivos `*_test.go` (ver roadmap). Esta varredura é
> RELATÓRIO — nenhuma ocorrência fora dos 5 arquivos foi editada.

## Parte 1 — Os 9 residuais (`#279`)

Confirmado por `git log -1 <arquivo>`: dos 4 arquivos citados pela issue, só `update_test.go` foi
tocado pela #269/ML-4A (`e6f0d83`), e mesmo ali a mudança converteu **um** símbolo diferente do
residual apontado. Os outros 3 (`scaffold_doctor_test.go`, `manager_persistence_order_test.go`,
`manager_test.go`, `provenance_test.go` — 4 arquivos, não 3) têm último commit anterior à #269.

O padrão do ML-4A (`internal/generators/execbit_probe_test.go`) é: **medir a condição real** (não
presumir por `runtime.GOOS`), rodar o resto do teste, e — só na porção que depende da propriedade
não representável — substituir o assert por uma mensagem nomeada em `os.Stderr` (não `t.Logf`:
`go test`/`t.Logf` bufferizam e descartam a saída de um pacote que passa sem `-v`, achado medido no
próprio `execbit_probe_test.go` — ressalva: o comentário desse arquivo ainda descreve isso como
"lacuna reportada, não fechada", mas `.github/workflows/quality.yml:390` já roda o job de Windows
com `-v` **desde o mesmo commit** `e6f0d83`/#269 que escreveu o comentário — dívida de documentação,
não gap real; ver a nota do vault deste ML). Manter `os.Stderr` nos dois novos probes deste ML é
consistência de idioma com o padrão já estabelecido, não compensação de uma lacuna ainda aberta.

Dos 9, o padrão se aplicou **integralmente** a 6, **não se aplicou** a 1 (removido, não migrado —
guarda espúria) e **não foi transplantado com o mesmo mecanismo** em 2 (classificados abaixo, com
justificativa).

| # | Local | Tratamento | Por quê |
|---|---|---|---|
| 1 | `scaffold_doctor_test.go:102` (`TestWrongModeDetection_ValidateScript`) | ✅ `execBitRepresentavelPara`/`execBitNaoExercitado` | Assert depende do bit de execução ser representável no FS real (não da suspensão por `CurrentGOOS` — ver nota abaixo) |
| 2 | `scaffold_doctor_test.go:133` (`TestWrongModeDetection_StaticScript`) | ✅ idem | idem |
| 3 | `scaffold_doctor_test.go:164` (`TestWrongModeDetection_ContentDivergence_TakesPrecedence`) | ❌ **skip removido, sem substituto** | `checkScaffoldArtifact` testa `bytes.Equal(actual, expected)` **antes** de qualquer checagem de bit (`scaffold_doctor.go:373` precede `:385`) — o assert (`DoctorScaffoldDivergent` vence) nunca toca o bit de execução, em nenhuma plataforma. O skip era cópia-e-cola das 3 vizinhas, não uma dependência real. Falsificado: mutei o produto para pular o `bytes.Equal` e o teste reprovou; revertido, passa — não era vácuo |
| 4 | `scaffold_doctor_test.go:215` (`TestExecBitPresent_UmaskNarrowedMode_Accepted`) | ✅ `execBitRepresentavelPara`/`execBitNaoExercitado` | idem 1/2 |
| 5 | `update_test.go:35` (`symlinkOrSkip`) | **NÃO ALTERADO — já dentro da decisão** | Ver "Parte 1-bis" abaixo |
| 6 | `manager_persistence_order_test.go:121` | ✅ `permissionEnforcementRepresentavel`/`NaoExercitado` (novo probe, mesmo idioma) | Ver "Parte 1-bis" |
| 7 | `manager_persistence_order_test.go:201` | ✅ idem | idem |
| 8 | `manager_test.go:217` (`TestManagerRejectsSymlinkFileAndParent`) | ✅ convertido de `runtime.GOOS` para `symlinkOrSkip` (idioma já existente, portado do pacote `generators`) | Ver "Parte 1-bis" |
| 9 | `provenance_test.go:154` | ✅ `permissionEnforcementRepresentavel`/`NaoExercitado` | idem 6/7 |

### Parte 1-bis — por que 3 dos 9 não usam a MESMA forma (probe + assert condicional)

**`scaffold_doctor_test.go` (itens 1/2/4) é diferente de `manager_test.go`/`manager_persistence_order_test.go`/`provenance_test.go`/`update_test.go` numa distinção que só apareceu ao ler a produção:**

- Em `scaffold_doctor.go`, a supressão do assert é uma **decisão de produto já aceita** (AC5,
  `CurrentGOOS != "windows"`, coberta por `TestWindowsPlatformGuard`) — o produto **também** suprime
  a checagem, incondicionalmente, quando `CurrentGOOS=="windows"`. Isso coincide, na prática, com
  "bit não representável em NTFS real", mas o discriminante certo continua sendo o FS (não o GOOS):
  um host WSL gravando num bind-mount NTFS (`/mnt/c`) teria `CurrentGOOS=="linux"` (produto NÃO
  suprime) mas o bit continuaria não-representável (assert falharia por vácuo de plataforma, não
  por regressão real). O probe protege exatamente esse caso.

- Em `manager_persistence_order_test.go`/`provenance_test.go`, o teste força uma falha de escrita
  via `os.Chmod(dir, 0o500)` — isso não é uma propriedade de UM arquivo já criado (como o bit de
  execução), é o **mecanismo inteiro de construção do cenário**: sem o enforcement, `manager.Update`
  simplesmente **sucede**, e não há nenhum "resto do teste" que sobreviva independente disso. Por
  isso o probe roda **antes** de qualquer setup (mede primeiro, constrói depois), e a branch
  negativa não deixa nenhum assert real rodar — só nomeia a garantia via `os.Stderr` e retorna. É a
  MESMA disciplina de medição do ML-4A, aplicada a uma propriedade diferente (enforcement de escrita
  em vez de bit de execução) — não um padrão novo.

- Em `update_test.go:35` (`symlinkOrSkip`, pré-existente desde o #221, anterior ao ML-4A) e agora
  também em `manager_test.go:217`: aqui a garantia sob teste **é o próprio symlink**. Não existe
  "resto do teste" que rode sem ele — diferente do bit de execução (propriedade de um arquivo já
  existente) e mais parecido com o caso de permissão acima. A diferença para o caso de permissão é
  que aqui **já existe** um mecanismo de detecção por CONDIÇÃO (não por `runtime.GOOS`):
  `isSymlinkPrivilegeError` tenta o `os.Symlink` de verdade e só classifica como "não exercitável"
  o erro específico (WinError 1314 / permission-denied) — qualquer outro erro é `t.Fatalf`, não
  suprimido. Decidi **manter `t.Skip`** (não converter para `os.Stderr` + `return` silencioso) nos
  dois casos, porque a alternativa trocaria um status `SKIP` explícito — visível em `go test -v`
  e em `go test -json` independente de `-v` — por um `PASS` que não verificou nada — **menos
  transparente**, não mais. `manager_test.go:217` estava usando `runtime.GOOS=="windows"`
  (presunção), então portei o mecanismo já validado de `generators.symlinkOrSkip` para
  `integrations`, no próprio `manager_test.go` (sem arquivo novo — os 5 arquivos de escrita
  designados pelo handoff continuam sendo exatamente os 5; os helpers novos vivem dentro deles,
  não em arquivos `_probe_test.go` extras como na primeira versão desta entrega) — mesmo padrão,
  pacote diferente, não invenção.

**Nenhum dos 9 caiu no caso "o teste estava certo e o produto está errado"** — todos os 9 assertam
comportamento que a produção já implementa deliberadamente (AC5 do scaffold doctor, ADR-2026-08-18
da ordem de persistência, e o guard anti-symlink do #189/#212).

## Parte 2 — Varredura do acervo inteiro (entregável principal)

Padrões buscados: `t.Skip`/`t.Skipf`/`t.SkipNow` (Go), `.skip(`/`it.skip`/`describe.skip` (Node,
`grep -a` para não pular `npm/src/validator/index.js`, que `file` classifica como binário — achado
antigo do vault, hoje o arquivo está de volta como texto UTF-8), `pytest.mark.skip(if)`/`pytest.skip`/
`unittest.skip` (Python), e early-returns por `runtime.GOOS`/`process.platform`/`sys.platform`/
`os.name` em arquivo de teste (nenhum encontrado sem marcador de skip associado, nos 3 runtimes).

### Contagem por runtime (após este ML)

| Runtime | Ocorrências | Antes deste ML (medido) |
|---|---|---|
| Go | 27 chamadas reais de `t.Skip*` (comando exato: `grep -rn "t\.Skip(\|t\.Skipf(\|t\.SkipNow(" --include="*.go" . \| wc -l` → 29 linhas, 2 das quais são comentários mencionando `t.Skip(...)` em prosa, não código — `barrier_contract_test.go:8` e `scaffold_doctor_test.go:167`, este último escrito por este ML) | 34 chamadas reais antes deste ML — medido por `git diff`: este ML removeu 8 chamadas (`t.Skip*`) e acrescentou 1 (`manager_test.go`, `symlinkOrSkip`, condition-based), líquido **-7** |
| Node | 1 | 1 (inalterado — não é um dos 5 arquivos deste ML) |
| Python | 9 (1 `pytest.skip` + 8 `@pytest.mark.skipif`) | 9 (inalterado) |

Note que a contagem "antes" do inventário do vault (`goos-guard...2026-09-01`, medido em Windows)
reportava `go 41`, pré-#269; a #269 fechou o grupo de 22 (ML-4A) + 4 (bash por caminho absoluto) +
outros, e este ML fecha 8 dos 9 residuais restantes (1 removido por ser espúrio). Os números desta
tabela são da árvore **atual** (macOS, POSIX), medidos por `grep`, não por execução em Windows —
mesma ressalva que o próprio vault já registra: contagem por leitura de mensagem/forma, não por
execução real em NTFS.

### Classificação completa

**LEGÍTIMO** (a propriedade genuinamente não existe/não é medível naquela plataforma ou ambiente, e
o guard nomeia a garantia em vez de escondê-la em silêncio):

- Os 8 dos 9 residuais convertidos nesta wave (itens 1,2,4,6,7,8,9 da tabela acima) + item 5
  (`update_test.go:35`, já dentro da decisão, condição-based).
- `internal/validator/validator_test.go:1118`, `internal/config/config_paths_test.go:120,146`
  (`os.UserHomeDir()`/`homedir.Dir()` falhou) — dependência de ambiente ($HOME ausente), condição
  medida via erro real, não GOOS.
- `internal/integrations/manager_persistence_order_test.go:120,201`,
  `internal/thirdparty/provenance_test.go:153` (`running as root`) — medido via `os.Geteuid()==0`,
  condição real (root ignora bits de permissão em POSIX), fora do escopo dos 9 (classe "ambiente",
  não "plataforma" — não fazem parte da lista original da #279 e não foram tocados).
- `internal/generators/roadmap_test.go:1245` (`chmod 0444 não bloqueia escrita como root`) — mesma
  classe, condição medida.
- `internal/serve/serve_test.go:66` / `pypi/tests/test_serve_address.py:89` (IPv6 loopback
  indisponível) — condição medida via tentativa real de bind/dial, não por SO.
- `internal/generators/tz_test.go:37,41,51` — disponibilidade do banco de fusos IANA no host,
  condição medida (`time.LoadLocation`), não GOOS.
- `pypi/tests/test_atomic_write_windows_fallback.py:106,132,156` (`skipif not _HAS_FCHMOD`) —
  gated em `hasattr(os, "fchmod")`, não em nome de plataforma; comentário do próprio arquivo já
  documenta a escolha ("nothing to spy on... not gated on platform name").
- `internal/commands/ship_test.go:1182` (`-short`) — idioma padrão do Go para testes de integração
  longos, ortogonal a portabilidade.
- `internal/commands/root_test.go:213`, `internal/commands/ship_test.go:1702`
  (`placeExecutableInPath`) — não são `t.Skip`: adaptam o MECANISMO por plataforma (extensão do
  binário falso; symlink vs. hardlink vs. cópia) para manter a MESMA asserção de segurança viva nas
  duas plataformas, documentado inline com o motivo medido (shim assinado do macOS, privilégio de
  symlink no Windows). Este é o padrão bom — adaptar, não suprimir.
- `pypi/tests/test_atomic_write_windows_fallback.py:90` (`if os.name == "posix":`) — a asserção
  exact-bits é POSIX-only por razão documentada no docstring (NTFS só honra o bit de escrita); o
  assert anterior (bytes gravados) já cobre Windows incondicionalmente.

**APAGA ASSERÇÃO** (nenhuma ocorrência encontrada nesta varredura, fora dos 3 já corrigidos por
este ML): nenhum `skip`/guard remanescente remove uma verificação em vez de adaptá-la ou nomeá-la.

**AMBÍGUO** (precisa de decisão explícita, fora do escopo desta ML — reportado, não corrigido):

- `internal/commands/branch_prune_test.go:423,605,720,878`, `internal/commands/ship_test.go:1203,1318`,
  `internal/generators/copilot_hooks_parity_test.go:33,64`,
  `pypi/tests/test_ship.py:1050`, `pypi/tests/test_branch_prune.py:347,502,586,692`
  (`git`/`node`/`python3` ausente no `PATH`) — **11 sítios**, classe "dependência ausente" já
  nomeada no inventário do vault (`goos-guard...2026-09-01`): o próprio vault argumenta que
  `skip` é mais grave aqui do que na classe plataforma, porque a ausência é **corrigível**, não uma
  propriedade do SO — "um teste pulado não mede mais que um que não existe" — e recomenda o padrão
  já usado em `bash_path.py` (#267): falhar nomeando cada candidato tentado, nunca pular. **Não
  tratado aqui** porque a #279 e este ML escopam só a classe "plataforma" (os 9); a classe
  "dependência ausente" é maior (11 sítios, 3 runtimes) e merece REQ própria.
- `internal/commands/push_test.go:255` — `t.Skip` incondicional (não platform-gated): o corpo
  inteiro do teste é a declaração de uma lacuna de cobertura já documentada em
  `docs/cli-parity.md` (anotação `partial=`). Não é um guard de plataforma nem "apaga asserção"
  (nunca houve assert), mas também não é claramente "legítimo" no sentido deste ML — é uma decisão
  de escopo de cobertura já registrada em outro lugar. Reportado como observação, sem ação.
- `internal/commands/barrier_contract_test.go:8-10` — comentário de cabeçalho **desatualizado**:
  descreve "cada teste chama `t.Skip(...)` como primeira linha", mas `grep` confirma **zero**
  `t.Skip` reais no arquivo hoje (9 funções `Test*`, todas com corpo executando). O ML-2A citado no
  próprio comentário aparentemente já rodou e removeu os skips, mas ninguém atualizou o texto que
  descreve o "mecanismo de pendência". Não é um `skip` (portanto fora da contagem acima), é dívida
  de documentação — sinalizado para quem tocar o arquivo em seguida.

## O que esta medição não prova

- Contei e classifiquei por leitura + `grep -rn`/`grep -a`, rodando em macOS (POSIX). Não tenho
  Windows real disponível nesta sessão — os 8 sítios convertidos foram falsificados por mutação de
  produção (produto quebrado → teste reprova; produto restaurado → teste passa), não por execução
  real em NTFS. A branch positiva dos novos probes (`execBitRepresentavelPara`,
  `permissionEnforcementRepresentavel`) roda e passa aqui porque APFS representa bit de execução e
  aplica enforcement de permissão — a branch negativa (NTFS) nunca foi exercitada nesta sessão.
- Não instalei node/python3 para confirmar os sítios de "dependência ausente" — classificação por
  leitura de mensagem, como o próprio vault já registrou como limite.
- `go build ./...`, `go vet ./...` e os testes dos 3 pacotes tocados (`generators`, `integrations`,
  `thirdparty`) rodam limpos aqui; não rodei `make quality` (fora do escopo desta ML, conforme
  handoff).
