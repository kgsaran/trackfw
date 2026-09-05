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

---

## Inventário COMPLETO dos achados, e onde cada um foi parar

🔴 **Esta seção existe porque a minha primeira leitura do parecer contemplou 6 dos 15 achados.** O
usuário perguntou "você contemplou todos?" e a resposta era **não**. Um parecer lido por alto vira
um parecer perdido — e o auditor já tinha perdido o arquivo dele para a janela de contexto.

| # | Achado | Onde está tratado |
|---|---|---|
| 1 | Teste do `ENOTDIR` reprova no Windows (`#276`) | ✅ REQ da auditoria, ML-1A |
| 2 | `#278` fechado cobrindo menos que o título | ✅ REQ da auditoria, ML-2A |
| 3 | Comentário do `#274` propõe discriminante que não discrimina | ✅ REQ da auditoria, ML-1B + ADR D3 |
| 4 | `continue-on-error` deixa regressão passar (`#275`) | ✅ ADR do ratchet |
| 5 | "ML concluído não pode afirmar o que o teste contradiz" | ✅ REQ da auditoria, AC6 / ML-3A |
| 6 | REQs com `adr:` vazio apontando para ADRs aceitos | ✅ REQ da auditoria, AC7 / ML-3B |
| 7 | **`#261` não tem REQ nem ADR — nenhum** | 🔴 **LACUNA — nada existia, e eu não criei** |
| 8 | **`#268`: AC1 revisado mas SEM implementação no contador Python; `sync` mantém `docs/req` fixo** | 🔴 **LACUNA — REQ existe, a implementação não** |
| 9 | **`#258`: evento `edited` + contrato próprio para exemplo citado** | 🔴 **LACUNA** |
| 10 | **`#273`: recomendação de vínculo explícito branch↔roadmap como opção principal** | 🟡 REQ existe, aberta; a **recomendação** não está registrada nela |
| 11 | **REQ de home divergente: AC1 (medir consumidor) e AC4 (avisar por divergência de variáveis) são condições DIFERENTES e se contradizem** | 🔴 **LACUNA — inconsistência interna de REQ nossa** |
| 12 | **REQ de ancestrais: resolver só o pai imediato não cobre múltiplos ancestrais inexistentes, nem junctions** | 🔴 **LACUNA — insuficiência de solução já aceita** |
| 13 | CRLF alcança **renderizadores**, não só o parser; ML-5A subespecificado | 🟡 Declarado fora de escopo na REQ da auditoria; **sem REQ própria ainda** |
| 14 | Jornada de instalação: README não qualifica dependência de shell; `install.sh` só Linux/macOS; Windows ARM64 fora da distribuição; jornada init→PATH→hooks→barrier não verificada | 🟡 Declarado fora de escopo; **sem REQ própria ainda** |
| 15 | Release publicada é a **v7.3.0 de 28/08** — anterior a tudo; correção na `main` não atende quem instala | 🟡 Decisão do usuário; registrado, sem artefato |

### O que muda por causa deste inventário

**Os itens 7, 8, 9, 11 e 12 são lacunas de verdade** — não "fora de escopo por decisão", e sim
**esquecidos**. Dois deles (11 e 12) são piores que os outros: apontam **defeito dentro de REQs
nossas já aceitas** — uma com ACs que se contradizem, outra com solução insuficiente para o caso que
ela mesma descreve.

**Os itens 13, 14 e 15 são fora-de-escopo legítimos**, mas fora-de-escopo **sem artefato** é o mesmo
que esquecido daqui a duas semanas. Precisam de REQ própria ou de registro explícito de adiamento.

🔴 **O item 15 é o que decide a utilidade de todo o resto:** a última release é anterior à campanha
inteira. Nada disso chegou a quem instala o trackfw.
