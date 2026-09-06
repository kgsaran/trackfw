# Leitor único fecha a classe, mas 2 abort viviam fora da família guard (mesmo defeito, 2 lugares que o ML-1C não contava)

**ROADMAP:** ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio, ML-1G
**Squad:** apolo-tf

## O que foi medido

O ML-1C tinha contado "Go 20 sítios de `os.ReadFile(` em `internal/validator/*.go`" e classificado a
resposta como arquitetural (fora do escopo daquele ML). O ML-1G roteou os 20 (mais os 4 sítios de
`internal/validator/validator_traceid.go` que a contagem já incluía, e o `readFileForRule` em si) —
7 deles eram sítios de REGRAS REAIS com `continue`/`except: pass` silencioso (`req_has_adr`,
`req_has_roadmap`, `frontmatter_presence` x2, `req_roadmap_lifecycle`, `folder_status`, `note_orphan`,
4x traceid) que escapavam de TODA verificação de governança quando o arquivo não podia ser lido —
exatamente a classe "fail-open" que esta REQ existe para fechar, só que fora da família guard.

## O que a contagem do ML-1C não pegou (achado por medição, não por enumeração)

**1. `validateCredentialGuardModeDowngrade` (Go, `validator_credential_guard_integrity.go`) —
estava DENTRO de um arquivo da família guard, mas fora das FUNÇÕES que 1A-1C corrigiram
(`*HookResolvable`, `*ScriptIntegrity`).** Um erro de leitura não-ENOENT (permissão negada, FIFO)
fazia `return nil, fmt.Errorf(...)`, que os dois call sites em `validator.go` propagam como
`return nil, nil, e` — ABORTA `trackfw validate` inteiro (stdout vazio, nenhuma outra regra roda).
Mesmo defeito que ML-1C fechou para `*_script_integrity`, sobrevivendo porque 1A-1C corrigiram
FUNÇÕES específicas, não todo raw read dos arquivos da família. Node já fazia a coisa certa aqui
(`inspectionDiagnostic` em vez de `throw`) — Go foi trazido à paridade, não inventou desenho novo.
Python tinha uma TERCEIRA variante do mesmo bug: tratava QUALQUER `OSError` (não só
`FileNotFoundError`) como se fosse "arquivo deletado, confirma o downgrade" — acusando um downgrade
CONFIRMADO quando na verdade só se sabe que a leitura falhou (pior que abort: é um FALSO POSITIVO
com alta confiança implícita). As 3 correções agora convergem: FileNotFoundError = downgrade
confirmado; qualquer outro erro = diagnóstico ("could not be read"), nunca abort, nunca
confirmação indevida.

**2. `validateThirdPartyArtifactHasProvenance` → `integrations.LoadManifest` (Go,
`internal/integrations/manifest.go`) — está FORA de `internal/validator` inteiramente, então a
contagem por pacote do ML-1C nunca a viu.** Mesmo assim é alcançada por uma regra real
(`thirdparty_artifact_has_provenance`) e tinha o MESMO abort: `fmt.Errorf` não-ENOENT propagado
como `return nil, nil, e`. Node e Python tinham a MESMA variante (`throw new Error(...)` /
`raise RuntimeError(...)`) no ponto equivalente. Fix: o call site de VALIDATE (não o `LoadManifest`
em si, que continua fail-closed para os consumidores de escrita — `trackfw thirdparty install`)
converte qualquer erro não-ENOENT em diagnóstico, nos 3 runtimes.

## Por que isso importa para o próximo agente

Ao enumerar "sítios de leitura crua de uma família X", uma contagem por PACOTE (Go) ou por ARQUIVO
(Node/Python) não é suficiente — uma função da família pode: (a) estar num arquivo-irmão mas fora do
conjunto de funções já corrigido, ou (b) estar inteiramente fora do pacote/arquivo mas ainda
alcançável por uma regra da família via uma chamada indireta (aqui, através de outro pacote). A
enumeração correta é por REGRA (`applyRule`/`applyRuleTagged` na Go, `applyRule` no Node/Python),
seguindo a cadeia de chamadas até o(s) `open`/`ReadFile`/`readFileSync` real(is), não por
`grep` no arquivo onde a maioria dos sítios mora.

## O gate novo

`scripts/check-raw-read-ban.sh` — bane `os.ReadFile(`/`fs.readFileSync(`/`open(` fora de um
allowlist inline (`raw-read-allowed: <razão>` na mesma linha ou na linha imediatamente anterior) em
`internal/validator/*.go` (não-teste), `npm/src/validator/index.js` e `pypi/trackfw/validator.py`.
Guarda de vacuidade agregada (mínimo 200 linhas somadas por runtime, não por arquivo — arquivos
utilitários pequenos como `regularfile_windows.go` são legítimos e não devem falhar sozinhos).
`grep -a` obrigatório no arquivo Node (classificado binário por `file(1)`); pattern Python evita
lookbehind PCRE porque o BSD grep do macOS não tem `-P` — usar `grep -P` ali reproduziria a MESMA
armadilha de gate vácuo desta campanha (erro engolido por `|| true`, "0 raw sites" mentiroso),
medido ao vivo antes de trocar para o padrão portátil `(^|[^A-Za-z0-9_.])open\(`.

Falsificado nas duas direções, nos 3 runtimes + guarda de vacuidade, manualmente (backup/sabotage/
restore via `cp`+`python3 -c`, não integrado a `check-gates-falsify.sh` — esse arquivo é escopo do
ML-1E/1F e cobre paridade de `trackfw validate`, não a superfície de código-fonte deste gate).

## O que ficou declarado, não corrigido

`internal/integrations/manifest.go:51` (`loadManifest`) continua com `os.ReadFile` cru — um FIFO em
`.trackfw/integrations-manifest.json` ainda travaria `trackfw validate` indefinidamente via
`LoadManifest`. O ABORT foi fechado (call site de validate agora diagnostica em vez de abortar), mas
o HANG não — `readRegularFile` mora no pacote `validator` e `internal/integrations` não o importa
hoje; portar o primitivo exigiria extraí-lo para um pacote compartilhado, decisão arquitetural fora
do orçamento deste ML.
