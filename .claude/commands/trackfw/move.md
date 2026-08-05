Execute o seguinte comando bash: `trackfw roadmap move $ARGUMENTS`

O formato esperado é: `<nome-do-roadmap> <estado>`

Estados válidos: `backlog`, `analyzing`, `wip`, `blocked`, `done`, `abandoned`

Exemplo: `/trackfw:move meu-roadmap analyzing`

Se o comando falhar com `trackfw: command not found` ou similar, informe ao usuário:
trackfw não está instalado. Instale com:
  curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
  npm install -g trackfw
  pip install trackfw