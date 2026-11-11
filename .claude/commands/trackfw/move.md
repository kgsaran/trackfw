Run the following bash command: `trackfw roadmap move $ARGUMENTS`

Expected format: `<roadmap-name> <state>`

Valid states: `backlog`, `analyzing`, `wip`, `blocked`, `done`, `abandoned`

Example: `/trackfw:move my-roadmap analyzing`

If the command fails with `trackfw: command not found` or similar, inform the user:
trackfw is not installed. Install with:
  curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
  npm install -g trackfw
  pip install trackfw