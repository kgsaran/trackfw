Execute o seguinte comando bash: `trackfw req new "$ARGUMENTS"`

⚠️ Em projetos com `roadmap_namespacing: by_agent` e 2+ agentes, use: `trackfw req new --agent <seu-agente> "$ARGUMENTS"`

Se o comando falhar com `trackfw: command not found` ou similar, informe ao usuário:

```
trackfw não está instalado. Instale com uma das opções:

  curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
  npm install -g trackfw
  pip install trackfw
```