---
name: "source-command-trackfw-validate"
description: "Migrated source command `trackfw-validate`"
---

# source-command-trackfw-validate

Use this skill when the user asks to run the migrated source command `trackfw-validate`.

## Command Template

Execute o seguinte comando bash: `trackfw validate`

Se o comando falhar com `trackfw: command not found` ou similar, informe ao usuário:

```
trackfw não está instalado. Instale com uma das opções:

  curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
  npm install -g trackfw
  pip install trackfw
```
