---
name: mudar-contrato-auditar-quem-afirma-o-antigo
description: Ao mudar um contrato de saída, enumerar os TESTES que afirmam a forma antiga — não só os consumidores de produto
metadata:
  type: feedback
---

🔴 **Ao mudar o contrato de retorno de uma função, enumerar os consumidores NÃO basta — enumere
também os testes que afirmam a forma antiga.** São duas varreduras, não uma.

**Why:** em 2026-09-28 (PR #464) mudei `roadmapCandidateFiles` para devolver separador POSIX. Eu
tracei os 4 consumidores de produto, classifiquei cada um, e escrevi no handoff que 3 eram imunes e
1 *"imprime o caminho cru ao usuário"*. Estava certo. **E mesmo assim o CI reprovou de novo**, porque
o teste desse quarto consumidor fixava a mensagem com `filepath.Join` — a forma **nativa**, o
contrato antigo. Custou um ciclo inteiro de CI e um segundo despacho.

A régua enxergou o consumidor e **não os testes do consumidor**. É o mesmo defeito de escopo estreito
que eu passei a campanha cobrando dos executores.

**How to apply:** antes de despachar mudança de contrato de saída, rode as duas varreduras e cite
ambas no handoff:

```bash
grep -rn "<função>" --include='*.go' .                    # consumidores
grep -rn "filepath.Join\|filepath.Base\|ToSlash" --include='*_test.go' <pacote>/  # quem fixa forma
```

**E o atalho que vale mais que as duas:** rodar a suíte inteira na plataforma onde o contrato muda,
antes do push. Foi o que finalmente deu a lista completa — uma falha real, não uma por ciclo de CI.

⚠️ **Mas leia o resultado com desconfiança**: a suíte na VM acusou 87 falhas únicas, 86 delas por
`exec: "bash": executable file not found in %PATH%` (o `cmd.exe` da VM não tem bash, o runner do CI
tem). Cruze sempre com `.github/windows-known-failures.json` **e** com o veredito do CI antes de
chamar qualquer uma de regressão. Ver [[o-instrumento-mente]].

Relacionado: [[medir-com-a-regra-nao-com-grep]], [[vm-investiga-ci-mede]].
