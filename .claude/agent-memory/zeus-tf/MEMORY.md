# Memory Index — trackfw (Zeus)

- [Verificação visual obrigatória em MLs de UI](feedback_verificacao_visual_obrigatoria.md) — gates verdes não provam UX; auditar em navegador real
- [Atribuição de papéis em MLs](feedback_atribuicao_de_papeis_ml.md) — Hefesto e Hades não modificam código, nem em cópia mecânica
- [Dashboard do serve é light-only](project_dashboard_light_only.md) — sem prefers-color-scheme; static do Go é go:embed e fonte canônica
- [trackfw ainda sem usuários downstream](project_trackfw_sem_usuarios_downstream.md) — compatibilidade retroativa não é restrição de peso; corrigir na origem
- [Push e PR passam por trackfw ship](project_push_e_pr_via_ship.md) — git push/commit brutos bloqueados por hook; ship exige algo staged; binário instalado costuma estar velho
- [Gate do Hades em artefatos de terceiro](project_gate_hades_artefatos_terceiro.md) — skill/agent/plugin: gate de runtime recorrente, quarentena → parecer → instala
- [Fronteira de escrita dos auditores](feedback_hefesto_recusa_docs.md) — Hefesto/Hades/Atena escrevem docs designadas; corrigido no gerador (#165), exige `trackfw agents update`
- [Verbosidade das respostas](feedback_verbosidade_das_respostas.md) — curto por padrão; detalhe só em bloqueio, decisão ou erro meu
- [Ler a REQ, não o ADR vizinho](feedback_ler_a_req_nao_o_adr_vizinho.md) — antes de dizer "fora de escopo", abrir a REQ que governa a feature; AC marcado que contradiz o medido é bug
- [Não narrar despacho antes de fazer](feedback_nao_narrar_despacho_antes_de_fazer.md) — "despachado" só depois da chamada do Agent no mesmo turno; aconteceu 2x
- [Não trocar de branch com agente vivo](feedback_nao_trocar_de_branch_com_agente_vivo.md) — nada de checkout/commit de outra branch com subagente editando; cópia durável agora, commit depois
