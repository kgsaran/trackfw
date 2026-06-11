# Roadmap: Slash Commands (Claude Code + Gemini CLI)

> Criado em: 2026-06-11 | Status: ✅ Done

## Contexto

Expor os comandos do `trackfw` como slash commands `/adr`, `/req`, `/roadmap`, `/validate` e `/status` nos assistentes de IA Claude Code e Gemini CLI. Quando o binário não estiver instalado, o comando falha com mensagem de instalação (não simula comportamento).

**Estrutura de saída:**
```
.claude/commands/    → slash commands do Claude Code
.gemini/commands/    → slash commands do Gemini CLI
```

**Formato Claude Code:** arquivo `.md` em `.claude/commands/` — conteúdo é o prompt enviado ao agente, `$ARGUMENTS` é substituído pelo texto após o comando.

**Formato Gemini CLI:** idêntico, em `.gemini/commands/`.

---

## Wave 1 — 10 arquivos em paralelo (2 ferramentas × 5 comandos)

**Status:** ⬜ Pendente

### Comandos a criar (mesma lógica para ambas as ferramentas)

| Arquivo | Comando CLI equivalente |
|---|---|
| `adr.md` | `trackfw adr new "$ARGUMENTS"` |
| `req.md` | `trackfw req new "$ARGUMENTS"` |
| `roadmap.md` | `trackfw roadmap new "$ARGUMENTS"` |
| `validate.md` | `trackfw validate` |
| `status.md` | `trackfw status` |

### Conteúdo padrão dos comandos com argumento (`$ARGUMENTS`)

```markdown
Execute: `trackfw <subcmd> new "$ARGUMENTS"`

Se o comando falhar por `trackfw: command not found`, informe:
"trackfw não está instalado. Instale com uma das opções:
  curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
  npm install -g trackfw
  pip install trackfw"
```

### Conteúdo padrão dos comandos sem argumento

```markdown
Execute: `trackfw <subcmd>`

Se o comando falhar por `trackfw: command not found`, informe:
"trackfw não está instalado. Instale com uma das opções:
  curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
  npm install -g trackfw
  pip install trackfw"
```

**Critérios de aceite:**
- [ ] `.claude/commands/` contém os 5 arquivos `.md`
- [ ] `.gemini/commands/` contém os 5 arquivos `.md`
- [ ] `$ARGUMENTS` presente nos comandos que precisam de título
- [ ] Mensagem de fallback menciona as 3 formas de instalação
