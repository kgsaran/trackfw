# Item 4 do harness precisa provar identidade do bash antes de invocar (2026-09-05)

## Contexto

O ML-2C (`ROADMAP-2026-09-05-retarget-dos-checks-de-camada-2-que-medem-a-plataforma-e-nao-o-produto.md`)
trocou o item 4 do harness de reprodução de Windows (`scripts/windows-repro/run.ps1`) de um mecanismo
replicado (`print()` isolado em `checks.py`) para uma invocação real de
`scripts/check-parity-contract-coverage.sh` via `bash`.

O retarget foi mesclado no PR #280 e o primeiro run em Windows real
(`windows-defect-reproduction`, run `33986718256`) deu **INCONCLUSIVE** — não confirmou nem refutou
o defeito, porque **não chegou a executar o script**.

## Causa raiz

`Run-Capture -Exe "bash" ...` resolve por **nome cru**. No runner do GitHub Actions,
`C:\Windows\System32\bash.exe` — o stub do WSL — atende pelo nome `bash` e, quando não há distro WSL
instalada, imprime `"Windows Subsystem for Linux has no installed distributions."` (em UTF-16 —
assinatura: texto com espaçamento caractere-a-caractere quando lido como texto de byte único) e sai
com código 1, sem executar nada.

Isto é a **mesma causa raiz** já diagnosticada no grupo B para os 50 testes Python (ver
[[system32-bash-exe-e-o-stub-do-wsl-e-vence-a-resolucao-por-nome-nu-2026-09-04]]): resolução por nome
cru é ambígua no Windows porque `CreateProcess` com `lpApplicationName=NULL` não segue a mesma ordem
de busca que `where.exe`/`shutil.which`, e o stub do WSL pode vencer.

🔴 **`env=` não resolve isto** — medido: o `CreateProcess` do Windows resolve nomes de executável sem
diretório pelo `%PATH%` do **processo pai**, nunca por uma variável de ambiente passada ao filho.

## O que isto NÃO é

Não é regressão do retarget do ML-2C. O retarget estava certo — ele passou a medir uma invocação real
em vez de um substituto, e a primeira coisa que a invocação real mediu foi que o ambiente do runner
tem uma ambiguidade de resolução de `bash` que o substituto escondia por construção (o substituto era
Python puro, nunca chamava `bash`). **Resultado bom, não regressão.**

## Correção

`scripts/windows-repro/resolve-bash.ps1` — função `Resolve-ProvenBash`:

1. Enumera candidatos por **caminho absoluto**: cada entrada do `%PATH%` + locais canônicos do Git
   for Windows (`C:\Program Files\Git\bin\bash.exe`, `...\usr\bin\bash.exe`, `...\usr\bin\bash`) +
   `C:\Windows\System32\bash.exe` (o stub — incluído de propósito, para ser testado e **reprovado**,
   não apenas evitado por convenção).
2. Para cada candidato, roda `<cand> --version` e só aceita se `exit=0` **e** a saída contém
   `"GNU bash"`. `shutil.which`/`where.exe` sozinhos não bastam como fonte única — é exatamente a
   resolução por nome que o stub vence.
3. Devolve o primeiro candidato provado, ou `$null` + a lista de tentativas (para diagnóstico).

O item 4 (`run.ps1`), quando `Resolve-ProvenBash` não encontra um candidato provado, reporta
`INCONCLUSIVE` com uma mensagem **nomeando cada candidato testado e por que foi reprovado** — nunca
cai no stub em silêncio (falsificado localmente: apontando o `%PATH%` só para um impostor que imita a
mensagem do stub, `Resolve-ProvenBash` devolve `Path=$null` com o candidato listado como
`proven=False`; com um bash real no `%PATH%`, resolve normalmente).

## Generalização

Qualquer invocação de `bash` por nome cru neste harness (ou em qualquer script que rode em Windows
real) carrega o mesmo risco. `pypi/tests/bash_path.py` já resolve isto do lado dos testes Python
(`bash_cmd`, gate de identidade em bytes). Este é o equivalente do lado do PowerShell — se outro item
do harness passar a invocar `bash` cru no futuro, deve reusar `Resolve-ProvenBash`, não reinventar.
