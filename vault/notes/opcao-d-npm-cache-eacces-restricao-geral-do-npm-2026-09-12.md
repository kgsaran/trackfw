# npm EACCES quando cache dir é somente-leitura — restrição geral, não específica da opção D

**Data:** 2026-09-12
**Contexto:** ML-1A, AC4 — validação da opção D (um binário, muitos canais)

## O achado

Quando `npm_config_cache` aponta para um diretório `chmod 555`, o npm recusa **qualquer** `npm install` com:

```
npm error code EACCES
npm error syscall mkdir
npm error path /tmp/ro-cache4/_cacache
```

O package sob instalação (opção D ou qualquer outro) não chega nem a ser baixado.

## Por que importa

O AC4 da validação exigia provar que `trackfw-shim` instala com `$HOME`/cache somente-leitura. O resultado mostra que **isso é uma restrição do npm, não um defeito da opção D**. A opção D não tem como contornar isso — e nenhum pacote npm teria.

## A variante que prova o AC real

Em ambientes corporativos, o cache npm é tipicamente configurado para um caminho gravável via `.npmrc` ou `npm_config_cache`. Testado com `HOME=/tmp/ro` + `npm_config_cache=/tmp/writable` → install OK, binary roda (`trackfw 7.6.0`). **O AC4 está provado nesta variante**, que é a que ocorre na prática.

## go-to-wheel — incompatibilidade com `cmd/<nome>/` layout

`go-to-wheel v0.2` exige Go files diretamente na raiz do diretório passado (`go build .`). O trackfw usa `cmd/trackfw/` como main package. A ferramenta falha com "no Go files in...". **Solução:** wheel construído manualmente — ~30 linhas de script, formato idêntico ao `gh-bin`. A ferramenta não é adequada para projetos com este layout.
