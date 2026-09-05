# Auditoria externa (GPT-6 Astra) — achados, e a verificação de cada um

> 2026-09-05 · registro feito por `trackfw_architect` (Zeus)
> Referência da auditoria: `main` em `4c69289` (PR #281 mergeado durante a análise).

## Por que este documento existe

O auditor **não conseguiu gerar o arquivo de análise** — estourou a janela de contexto e entregou as
conclusões no chat. Sem este registro, o parecer se perde e os achados voltam a ser descobertos do
zero daqui a um mês.

🔴 **Regra que aplico aqui, a mesma dos issues do consumidor externo:** um achado de auditoria é
**afirmação a verificar**, não veredito a acatar. Cada item abaixo tem o veredito *da minha
verificação*, com o comando que a produziu — inclusive quando confirma que erramos.

## Os três achados que verifiquei diretamente

### A1 — O teste que escrevemos para o `#276` REPROVA no Windows · **CONFIRMADO**

Log do CI do próprio PR #281 (run `33991655271`, job `windows-full-suites`):

```
=== RUN   TestDetectNameCollision_ENOTDIRIsReportedNotSwallowed
    manager_collision_enotdir_test.go:58: detectNameCollision(ENOTDIR) = nil,
    want a reported error (ENOTDIR must not be classified as absence)
--- FAIL
```

🔴 **É contradição interna nossa, não do auditor.** O ML-1C **mediu e escreveu** que `ENOTDIR` é
indistinguível de ausência no Windows (`ENOTDIR = ERROR_PATH_NOT_FOUND`) — e **mesmo assim** deixou
um teste afirmando o contrário. O ML foi marcado ✅ e eu mergeei sem cruzar o teste novo com a
conclusão do próprio ML.

**Introduzimos um vermelho novo na `main`.** É exatamente a classe de regressão que o ratchet
proposto no `#275` bloquearia no PR.

### A2 — O `#278` está PARCIALMENTE corrigido · **CONFIRMADO**

Medido com o código mergeado:

```
'ADR: docs/adr/x.md'              -> True    ✓ valor real
'ADR:\n'  ·  'ADR:   \n'          -> False   ✓ as 7 grafias vazias
'ADR: <!-- preencher depois -->'  -> True    ✗ placeholder conta como valor
'veja a secao ADR: mais abaixo'   -> True    ✗ prosa conta como valor
```

As **grafias de vazio** foram resolvidas. **Interpretar o vínculo, não.** A issue foi fechada pelo
PR #281 — o fechamento cobre menos do que o título dela sugere.

### A3 — O nosso comentário público no `#274` está ERRADO · **CONFIRMADO**

Reproduzido localmente:

```
teste que roda e reprova   → # tests 1 · pass 0 · fail 1
import quebrado            → # tests 1 · pass 0 · fail 1
```

**Idênticos.** Eu comentei na issue afirmando que `pass 0 / fail 1` distingue "a suíte não carregou".
**Não distingue.** O discriminante tem de ser o **tipo do evento** (erro de carregamento vs. assert),
não a contagem.

🔴 A ironia é dupla: a issue existe para dizer que *o CI não distingue estados*, e a nossa resposta
propôs um discriminante que **também não distingue**.

## Achados que aceito sem verificação independente (e por quê)

- **Release publicada é a v7.3.0, de 28/08** — anterior a toda a campanha. Verificável em um clique,
  e decisivo: *correção na `main` não atende quem instala*.
- **CRLF alcança renderizadores, não só o parser de validação.** O ML-5A está subespecificado para
  essa superfície. Consistente com o que já sabíamos (o `barrier` do Python tem um parser próprio —
  o "G1-bis" da re-triagem).
- **`install.sh` só aceita Linux/macOS**, embora haja binário Windows publicado; Windows ARM64 fora
  da distribuição; jornada instalação → PATH → init → hooks → barrier não documentada nem verificada.

## Achados sobre governança, que aceito e doem

- **"ML concluído não pode afirmar correção que o teste contradiz."** É exatamente o A1.
- **REQs devem apontar os ADRs já aceitos** — várias têm `adr:` vazio apontando para decisões que
  existem.

## O que a auditoria NÃO derruba

O parecer é **favorável à direção dos ADRs** — normalização na entrada, separação entre caminho de
arquivo e representação textual, shell POSIX explícito. O problema apontado não é a decisão: é
**transformá-la em comportamento comprovado de ponta a ponta**, com CI que impeça regressão e uma
release que entregue.

## O padrão que atravessa A1, A2 e A3

Nos três, **produzimos evidência e não a cruzamos com a nossa própria conclusão**:

| | medimos | e mesmo assim |
|---|---|---|
| A1 | `ENOTDIR` é indistinguível no Windows | escrevemos teste afirmando que é distinguível |
| A2 | 7 grafias de vazio | fechamos a issue como se o vínculo estivesse resolvido |
| A3 | — | afirmamos publicamente um discriminante que não testamos |

Não é falta de medição. É **falta de reconciliação entre o que medimos e o que declaramos**.
