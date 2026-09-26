# Memory Index — apolo-tf (trackfw)

- [Retomar execução parcial](feedback_retomar_execucao_parcial.md) — confiar no "já feito e verificado" do handoff, focar esforço só no delta listado como pendente
- [make quality excede o timeout da tool](project_make_quality_excede_timeout.md) — ~13 min, roda em background; exit code só sem pipe; markdown tardio só re-roda os gates de doc
- [Métrica por artefato, não por regra](feedback_metrica_por_artefato.md) — "a regra apareceu na saída" dá falso verde; exigir que a violação nomeie o artefato, e medir nas duas árvores
- [Classificar marcadores: ler RejectSymlinks antes de vereditar](feedback_classificar_marcadores.md) — confirmar se a função checa a folha ou só os ancestrais; define se 48 marcadores são (A) ou (B)
- [Refutações primeiro no relatório](feedback_refutacoes_primeiro_no_relatorio.md) — censo do handoff é régua de identificador e subconta o mecanismo; abrir com a divergência medida, não enterrá-la
- [Medir antes/depois sem git stash](feedback_medir_antes_e_depois_sem_git_stash.md) — `git archive HEAD | tar -x` + dois binários, exercitados sobre CÓPIA do corpus real
- [Grepar scripts inclui a LISTA DE ARGUMENTOS](feedback_grep_scripts_inclui_lista_de_argumentos.md) — cenário 24 fixa o texto do template, cenário 25 fixa os args do Sprintf; esquecer o 2º custa 13 min
