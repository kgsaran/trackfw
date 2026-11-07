# Zeus — fim da REQ do guard de credencial que atribuía ao payload o achado de arquivo referenciado

- **PR #672 mergeado.** Waves 0, 1 e 2 concluídas; `make quality` verde; `trackfw barrier` passou nas três; 22 checks verdes no CI, inclusive `windows-full-suites`.
- **Entregue:**
  - ML-1A: mensagem diz a origem ("detected in file <path> referenced by the command"), caminho sanitizado; glob relativo pela cwd da ferramenta, com fallback para `cwd` de raiz (Kiro/Claude Code);
  - ML-1B: fixtures montam o token em runtime, com censo que impede literal nos testes;
  - ML-1C: oráculo `.sh` e paridade do sinal de atenção (sem caminho);
  - ML-2B: corretivo do A-1 do red team — a `cwd` entra no glob como literal (`credGlobEscapeLiteral`).
- **AC3 medido na `main` pós-merge** com binário compilado dela: `grep -n foo internal/guard/*.go` e `grep -n foo internal/generators/*_test.go` dão rc=0 no guard; controle positivo com JWT montado em runtime dá rc=2.
- **Incidentes:**
  - o guard instalado (11.5.0) bloqueou cinco comandos do red team por cabeçalho PEM literal no texto do próprio comando;
  - o apolo-tf editou o status do roadmap no ML-2B, fora da lista fechada; o arquiteto sobrescreveu.
- **Pendente:** nada desta REQ. O falso positivo do guard instalado só some para quem atualizar o binário — entra na próxima release.
