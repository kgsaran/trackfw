---
status: Accepted
date: 2026-08-30
author: "trackfw_architect (Zeus)"
---

# ADR: CI de Windows como instrumento de medição — job largo que nasce vermelho, mais sonda sob demanda

> Date: 2026-08-30 | Status: Accepted

## Context

A issue #216, de **@lourivalgarciajunior**, reportou **sete** defeitos de Windows na v7.3.0, todos
com evidência medida. Verifiquei os sete contra o código: confirmam-se, inclusive as contagens.
Investigando, achamos **mais três** (CRLF na leitura, `sh -c` hardcodado no Go, postura divergente
com `\`), e ele reportou **mais três** em comentários (testes de symlink não executáveis sem
privilégio, `ref_targets_exist` vácuo em `by_agent`, separador de SO vazando para artefato).

**Nós temos um job de Windows no CI, e ele estava verde durante tudo isso.**

`.github/workflows/quality.yml:83` — `windows-integrations-resolve` roda **três invocações
dirigidas**: um `go test -run TestResolveWindowsCrossplatform`, um arquivo de teste do Node, um
arquivo do Python. O comentário do job é honesto sobre o propósito: guard de regressão para um bug
específico de normalização de path.

O defeito não é o job existir estreito — é **não existir mais nada**. Nenhuma suíte completa, nenhum
`validate`, nenhum `init` rodam em Windows. O projeto publica três CLIs e não sabe se funcionam lá.

**A consequência mais cara não é técnica.** Um usuário adotou a ferramenta, rodou tudo, mediu sete
defeitos e escreveu o relatório mais bem instrumentado que este projeto já recebeu. Todos existiam
porque o nosso CI não olha. Ele fez o trabalho que o nosso pipeline deveria ter feito.

**E o custo se acumula:** duas correções de segurança que entregamos esta semana — a guarda de
symlink no `update`/`discover` e a do `roadmap move` — têm testes que **não executam** em Windows
padrão, porque `os.symlink` exige Developer Mode. Doze testes vermelhos, nenhum sinal sobre a guarda
funcionar. Escrevemos defesa e não sabemos se ela vale onde o comportamento de link é mais ambíguo.

## Decision

**1. O job de Windows passa a rodar as três suítes completas**, não invocações dirigidas. É a única
forma de o CI responder "o trackfw funciona em Windows?" em vez de "aquele bug de path não voltou?".

**2. O critério de aceite do job é que ele NASÇA VERMELHO.** Antes das correções, o job largo tem
que **reprovar reproduzindo os defeitos da issue #216**. Um job de Windows que nasce verde não prova
nada — foi exatamente assim que chegamos aqui. A transição vermelho → verde, correção a correção, é
a evidência.

Isto inverte a ordem usual: o CI entra **antes** dos fixes, e a primeira execução é uma medição, não
um portão.

**3. Uma sonda sob demanda (`workflow_dispatch`), separada do job de regressão.** Ela responde
perguntas pontuais em minutos — qual modo o `os.Stat` devolve, o `Lstat` marca junction como
symlink, o `isatty` mente para `NUL`, o gerador escreve CRLF — e imprime o resultado bruto, que vira
evidência anexável em REQ.

**São ferramentas distintas e não se substituem:** sonda prova o que alguém lembrou de perguntar;
regressão prova que não voltou. Confundi-las é o mesmo erro do gate de falsificação que não pegou o
bypass de cerca — cobria a classe e não tinha o caso.

**4. Teste que exige privilégio faz `skip` com mensagem que nomeia a garantia não exercitada.**
Nunca falha silenciosa, nunca sumiço do relatório. A formulação é do próprio autor da issue, e o
motivo é dele: *"é diferente de ficar em silêncio"*. Um `skip` que só desaparece recria o problema
que este ADR existe para corrigir, em escala menor.

**5. Windows é plataforma suportada via Git Bash/WSL, e isso passa a ser explícito.** O trackfw gera
**cinco scripts `.sh`**, quatro com shebang `bash` — os guards de credencial e de branch entre eles.
Sem shell POSIX, controles de segurança do produto não executam. A dependência **já existe**; o que
não existe é o aviso. Formalizar isso é REQ própria, já acordada, e esta decisão apenas fixa a
postura.

## Consequences

**Positivas**
- O CI passa a responder a pergunta que os usuários fazem, e não a que era conveniente medir.
- A transição vermelho → verde vira evidência por correção, em vez de "confia que arrumei".
- A sonda encurta o laço de investigação de ~20 minutos de suíte para poucos minutos, e a saída é
  citável.
- O `skip` explícito transforma "não testado" em conhecimento, em vez de dívida invisível.

**Negativas e riscos aceitos**
- **O `main` fica vermelho até as correções entrarem.** É desconfortável e é o ponto: o vermelho já
  era verdade, só não era visível. Mitigação: o job largo entra como **não bloqueante** até a última
  correção, e só então vira obrigatório — senão paralisa todo trabalho não relacionado.
- Runner de Windows é mais lento e mais caro em minutos. Aceito: o custo de não ter é um usuário
  descobrindo por nós.
- A sonda é superfície nova de execução manual. Mitigação: só `workflow_dispatch`, sem segredos,
  sem escrita no repositório.
- **O runner não responde tudo.** Junction, `core.symlinks=false` e console cp1252 dependem da
  configuração da máquina de quem clona. Para essa classe, a validação continua dependendo de
  usuário real — e é por isso que o papel de validador foi proposto ao autor da issue.

## Alternatives Considered

**Aceitar o PR do fork dele.** Ele ofereceu, e o trabalho é bom. Rejeitada: a regra dura de paridade
faz cada correção tocar de 20 a 70 sítios; auditar isso linha a linha custa mais que escrever com
governança, e entraria sem ADR, sem modelo de ameaça e sem gate falsificável. O crédito da
descoberta é dele de qualquer forma, e o papel de validador aproveita o que só ele tem.

**Container Windows local.** Impossível no ambiente do arquiteto: container Windows exige host
Windows, e Docker em macOS roda VM Linux. E Wine daria respostas erradas com confiança justamente
nos pontos que interessam.

**Corrigir os defeitos primeiro e alargar o CI depois.** É a ordem intuitiva e a errada: sem o job
largo não temos como provar que as correções funcionam, nem que continuam funcionando. Seria
escrever correção validada no ambiente errado — precisamente o viés que
`vault/notes/ambiente-do-dev-e-mais-rico-que-o-do-ci-2026-08-29.md` registrou três vezes num dia.

**Só a sonda, sem job largo.** Barata e responde investigação. Rejeitada: não impede regressão. O
oitavo defeito voltaria, e voltaria invisível.
