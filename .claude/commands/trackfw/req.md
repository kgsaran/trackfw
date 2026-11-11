Run the following bash command: `trackfw req new "$ARGUMENTS"`

⚠️ In projects with `roadmap_namespacing: by_agent` and 2+ agents, use: `trackfw req new --agent <your-agent> "$ARGUMENTS"`

If the command fails with `trackfw: command not found` or similar, inform the user:

```
trackfw is not installed. Install with one of these options:

  curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
  npm install -g trackfw
  pip install trackfw
```