# Zeus — início da REQ do guard de credencial (origem do achado)

- **Situação:** REQ de CI vermelho encerrada (#669 e #670 mergeados, branches apagadas). Release não lançada: desde a v11.5.0 só mudaram testes e CI.
- **Nova REQ-2026-11-06 (guard de credencial)** na branch `fix/guard-de-credencial-origem-do-achado`, com ADR próprio. A segunda camada atribui ao payload o achado que veio de um arquivo referenciado, e bloqueia glob mas não `grep -r`. As fixtures do próprio repositório bloqueiam quem trabalha nele.
- **Próximo:** Wave 0 com o hades-tf; depois ML-1A (apolo-tf) e ML-1B (artemis-tf) em paralelo; por fim o red team.
