# Atenção nativa Go no Windows — P21, 2026-10-20

REQ: `docs/req/REQ-2026-09-05-a-instalacao-em-windows-promete-o-que-nao-entrega-e-o-instalador-recusa-a-plataforma-para-a-qual-publicamos-binario.md`

## Instrumento e fonte

- [Execução isolada P21](https://github.com/kgsaran/trackfw/actions/runs/39454688584): **success**, `attention_only=true`, fonte `20b3734c5c7e959b10bb377901b0b40c26724879`, runner `windows-latest`.
- Binário compilado do fonte no próprio job, SHA-256 `B8ED9E08517E5B9B3C0D26FE920DC966EB8E55670B8CF83EF396F18821B7931A`.
- Diretórios descartáveis com espaço/acento, configuração YAML CRLF e `roadmap_dir` relativo com espaço/acento. PATH dos filhos contém somente o diretório do binário; shells e Edge são chamados por caminho absoluto. Perfil do browser e home dos processos ficam no fixture.
- Resolução observada: `trackfw=True`; `bash=False`, `jq=False`, `python=False`, `python3=False`.

## Valores observados

| Superfície | Resultado bruto |
|---|---|
| PowerShell signal / cleanup | ambos exit 0, timeout false, stdout/stderr 0 bytes |
| PowerShell API após signal | active true, observed true, message_exact true |
| PowerShell API após cleanup | active false, observed true |
| cmd signal / cleanup | ambos exit 0, timeout false, stdout/stderr 0 bytes |
| cmd API após signal / cleanup | active true / false, observed true, mensagem exata |
| Edge após signal | exit 0, timeout false, banner_found true, visible true, message_exact true, observed true |
| Edge após cleanup | exit 0, timeout false, banner_found true, visible false, observed true |
| Arquivo dedicado de credential guard | bytes_identical true após os dois shells |
| Servidor antes do finally | running true; encerramento pelo handle próprio no finally |

O Edge carregou o board HTTP normalmente e seu JavaScript realizou o polling. Nenhuma função do board ou estado de atenção foi injetado no browser. O instrumento observou o DOM real, incluindo classe de visibilidade e texto decodificado; não é uma avaliação estética por screenshot.

## Alcance do aceite

**AC8 / ML-1H comprovado:** comandos nativos, sem os helpers POSIX/Python, produzem e limpam o sinal que ativa/oculta o banner no browser Windows. Entrada fixa contém Unicode, aspas e metacaracteres; é transmitida apenas por stdin.

**AC5–AC7 continuam pendentes.** `command_origin.kind=constant_command_smoke` e `generated_fixture=false`: a sonda usa as linhas canônicas conferidas no fonte, sem disparar eventos dos seis CLIs de agente. Os emissores/migrações são cobertos pelos testes Go; aceitação de schema, evento, cwd, confiança e PATH de cada agente real ainda exigem outra jornada. Não inferir essa cobertura do status verde do job.

## Execução completa anterior

A [execução 39453598085](https://github.com/kgsaran/trackfw/actions/runs/39453598085), fonte `658c07ab`, também confirmou P21 nativo/API nos dois shells, mas o workflow completo terminou **failure**, não aprovação geral do Windows. Falharam as perguntas 5c e 18:

- 5c invoca caminhos da implementação Node retirada: `Cannot find module '../src/lib/fatal-error'`.
- 18 reportou cinco testes que já constam em `.github/windows-known-failures.json`: `TestShip_Integration_GracefulDegradation_RealBinary`, `TestChecksum_StableAndMatchesSHA256Sum`, `TestStaleWIPReportsWIPWalkError`, `TestFolderStatus_DiretorioNaoLegivel_P2`, `TestFilenameUniqueness_DiretorioNaoLegivel_P2`.

Esses resultados não foram corrigidos nem ocultados nesta frente. O modo isolado evita executar sondas alheias ao ML, mantendo o fluxo completo como padrão. A aprovação local foi `make quality` exit 0, incluindo 8 chunks de falsificação, 368 OK/0 FAIL e guarda de conjunto completa. Os pareceres de qualidade e red-team liberaram o produto; nenhum arquivo do PR #590 foi alterado.
