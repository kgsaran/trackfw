# Três armadilhas ao escrever cenário em `check-gates-falsify.sh`

> 2026-08-12 · acumuladas nos ML-2A dos roadmaps de detecção e de prova negativa

## 1. Provar que a regra pode ser **desligada** não prova que o cenário **depende** dela

**A armadilha mais sutil, e a que passou por duas revisões.**

Para provar que um cenário de falsificação não é vácuo, o reflexo é: *"ponho a regra em
`rules: <nome>: off` e confirmo que a mensagem some"*. **Isso prova que o botão de configuração
funciona — não que a asserção de detecção dependa da regra.** Uma regra **morta** passaria nesse
teste exatamente igual.

**O que prova de verdade:** rodar o **critério exato do `assert_fails_with`** (exit != 0 **E**
mensagem presente) contra o fixture com a regra desabilitada, e exigir que **falhe**. Existe helper
para isso: **`assert_would_now_fail`** (`scripts/check-gates-falsify.sh:52`).

E, ao sabotar **de fora** para auditar, desabilitar a regra comentando a chamada **não compila** em
Go (`declared and not used`). Consuma a variável:

```go
_ = credentialGuardModeMsgs // sabotagem temporária
```

> ⚠️ **RESSALVA DE ESCOPO (2026-08-12, ML-2A do roadmap de ancoragem).** O idioma prescrito acima —
> desabilitar a regra via `rules: <nome>: off` **não commitado** — está **morto para as três regras de
> credential-guard** (`credential_guard_hook_resolvable`, `credential_guard_script_integrity`,
> `credential_guard_mode_downgrade`). Desde o `ADR-2026-08-12-severidade-das-regras-...`, elas
> resolvem severidade pela **mais estrita entre `HEAD` e disco**, então um `off` em disco **não
> desliga nada**. Para essas três, use `off` **commitado** (o caminho legítimo e auditável) ou sabote
> o call site no código. Para as outras ~38 regras, o idioma original continua válido.

## 5. Regras extras de `wip`/`blocked` disparam em fixture de roadmap

Ao montar fixture com roadmap em `wip/` ou `blocked/`, o validador aplica regras adicionais que não
existem nos outros estados. Um fixture escrito para exercitar **uma** regra pode reprovar por
**outra**, e o diagnóstico aponta para o lugar errado. Monte o fixture no estado mais neutro possível,
ou fixe as regras extras em `off` **e comente por quê**.

## 2. Regra de severidade `warning` **não pode** usar `assert_fails_with`

`assert_fails_with` exige **exit != 0**. Uma regra cujo default é `warning` **não muda o exit code**
do `validate` — o cenário passaria sempre, provando nada.

**Solução:** fixe a severidade no `trackfw.yaml` do fixture:

```yaml
rules:
  credential_guard_script_integrity: error
```

⚠️ E **comente no cenário por que** o fixture sobrepõe o default — senão o próximo leitor acha que é
inconsistência com o `ruleDefaults` e "conserta".

## 3. Fixture que usa `git commit` precisa isolar a config global

O primeiro fixture deste arquivo a commitar (Cenário 50) precisou de:

```bash
git config commit.gpgsign false
git config core.hooksPath /dev/null
```

Sem isso: um `commit.gpgsign=true` global falha por falta de chave, e um `core.hooksPath` global roda
hooks do usuário dentro do fixture. Passa na máquina de quem escreveu, quebra na dos outros — mesma
família do falso negativo de `$HOME` não isolado de 2026-08-08.

## E uma que é de auditoria, não de escrita

**Ao sabotar para verificar um cenário, reconstrua `bin/trackfw`.** `go build ./...` **não** regenera
esse binário, e os cenários o usam. Sabotar sem reconstruir testa o **binário antigo** e o cenário
"passa" — dando falso conforto de que ele é vácuo quando não é.

```bash
go build -o bin/trackfw ./cmd/trackfw
```

*(Zeus caiu nesta auditando o Cenário 47.)*

## Relacionado

- `scripts/check-gates-falsify.sh` — Cenários 46, 47, 48, 49, 50
- `vault/notes/check-agent-hooks-parity-unisolated-home-false-failure-2026-08-08.md` — mesma família:
  estado de ambiente contaminando gate
- `vault/notes/vies-do-tmp-ao-medir-sandbox-de-agente-2026-08-12.md` — idem, em medição
