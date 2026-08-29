---
title: trackfw update escrevia fora do projeto através de symlink em .github/workflows/
tags: [seguranca, update, discover, symlink, gotcha]
date: 2026-08-28
related: [[barrier-so-casa-cabecalho-de-aceite-em-portugues-2026-08-28]]
---

## O defeito (corrigido em 2026-08-28, mesma branch em que foi introduzido)

`trackfw update` sobrescrevia **arquivo fora do diretório do projeto**, através de symlink em
`.github/workflows/trackfw-validate.yml`. Reprodução:

```bash
T=$(mktemp -d); OUT=$(mktemp -d)
mkdir -p $T/.github/workflows
printf 'project: p\nhooks: none\nci: none\n' >> $T/trackfw.yaml   # + req_dir/roadmap_dir
echo "CONTEUDO ORIGINAL DA VITIMA" > $OUT/vitima.txt
ln -s $OUT/vitima.txt $T/.github/workflows/trackfw-validate.yml
cd $T && trackfw update
head -1 $OUT/vitima.txt      # → "name: trackfw validate"  ← sobrescrito
```

Com `discover --init` e symlink **pendurado**, criava um arquivo novo no caminho escolhido pelo
atacante.

## Causa Raiz

Nenhum dos 3 runtimes usava `lstat`. A presença era decidida por `os.Stat` / `fs.existsSync` /
`os.path.isfile` e a escrita por `os.WriteFile` / `fs.writeFileSync` / `open()` — **as duas pontas
seguem symlink**.

O gatilho foi ampliado por uma **regra de aceite minha** (AC17(c) da
`REQ-2026-08-28-gate-de-ci-gerado-instala-versao-nao-pinada...`): o alvo `ci-workflow` passou a
ativar pela **presença do arquivo em disco** em vez de por `cfg.ci`, para cobrir o projeto que rodou
`discover` com `ci: none`. Correto quanto à cobertura, e foi exatamente isso que pôs um projeto
`ci: none` — que optou explicitamente por ficar fora — dentro da superfície de escrita.

## Por que importa mais do que um write comum

`.github/workflows/` é o diretório mais sensível de um repositório: quem escreve nele controla o CI,
e quem controla o CI controla a máquina de quem der checkout. Um repositório hostil que traga o
symlink no diff faz um `trackfw update` do mantenedor escrever onde o atacante escolher.

## Correção

`Lstat` / `lstatSync` / `os.path.islink` para decidir presença, e **recusa explícita** de escrever
através de symlink, com aviso em stderr — byte-idêntico nos 3 CLIs:

```
aviso: .github/workflows/trackfw-validate.yml é um symlink; trackfw update não escreve através de
symlinks — arquivo não foi tocado
```

Recusar em silêncio foi rejeitado na auditoria: viraria *"o update não atualizou meu workflow e não
disse nada"*. A primeira entrega do Node era segura mas muda, e foi reprovada por isso (ML-3E).

## Lição

Nenhum gate pegaria: escrever **no lugar errado** passa em todo teste que verifica **conteúdo**.
Quem pegou foi a revisão de segurança sobre o **diff entregue** — não sobre o plano. A Wave 0
original olhava o `install.sh` e não tinha como prever um vetor que só nasceu no ML-2G/2H, dois
microlotes depois.

Ao ampliar o gatilho de uma escrita, reavalie a superfície: *cobertura maior é superfície maior.*
