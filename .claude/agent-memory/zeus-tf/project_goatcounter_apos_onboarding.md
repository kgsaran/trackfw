---
name: goatcounter-apos-onboarding
description: Sequência combinada em 2026-11-22 — onboarding → GoatCounter no site → site novo (protótipo primeiro); REQ do GoatCounter só após merge da REQ-2026-11-22
metadata:
  type: project
---

Em 2026-11-22 o KG escolheu **GoatCounter** como analytics do site (`site/`, GitHub Pages) e pediu a REQ **depois** da REQ-2026-11-22 (onboarding verificável).

**Why:** o post no LinkedIn da semana de 2026-11-15 trouxe só 3 visitas ao repo pelos referrers do GitHub, e o tráfego do GitHub Pages não é medido. GoatCounter não usa cookie (sem banner LGPD) e é grátis para uso não comercial. Cloudflare, Umami e GA foram descartados.

**How to apply:**
- Ao fechar o onboarding, abrir a REQ: script no `site/.vitepress/config.mts`.
- O KG precisa criar a conta e passar o código `<código>.goatcounter.com`; pedir isso antes da REQ.
- O site muda de raiz na REQ-2026-11-22 (inglês em `/trackfw/`, pt-BR em `/pt/`), então o script vale para os dois locales.
- Sugerir links com `utm_source=linkedin` nos posts.

**Sequência combinada com o KG (2026-11-22):** onboarding → GoatCounter (medir o tráfego atual) → **site novo**.
- O site novo terá imagens, cenários de uso (dev solo com agente, time com CI, empresa só com npm/pip) e o desenho do fluxo ADR → REQ → ROADMAP → waves → barrier.
- Segue o método de protótipo primeiro (§9 das regras globais): conceito da atena-tf → protótipo descartável validado visualmente pelo KG → ADR/roadmap.
- Imagens geradas a partir do binário, não feitas à mão.
