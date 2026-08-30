# ADR-0005: Mecanismo de Atualização de Plugins

**Status:** Aceito
**Data:** 2026-08-23
**Contexto de produto:** `docs/prd.md` — RF-19, RNF-3, RNF-4
**Governa:** `docs/add/add-0001-work-system-architecture.md`, Seção 12

## Contexto

O gatilho de atualização é comparar apenas um campo opaco de versão declarado pelo autor do plugin, nunca o histórico bruto de commits — um híbrido entre fixar uma referência exata (reprodutibilidade) e rastrear uma branch usando esse campo como gate (fricção mínima ao autor). Faltava definir (a) se a atualização pode ser automática, e (b) como checar a versão remota sem perturbar a cópia de trabalho atualmente instalada e em uso do plugin.

## Decisão

* Atualização é sempre explícita, nunca automática/silenciosa (RF-19). Existe um comando somente-leitura para listar o que está desatualizado, um comando de atualização individual e um de atualização em lote (mostrando o que vai mudar, com confirmação). `work start`/`resume`/`archive` nunca disparam nada disso (RNF-4).
* Checar atualização nunca toca a cópia de trabalho que serve o entrypoint atualmente instalado do plugin — só a consulta explícita e confirmada move o quê está de fato em uso para uma nova referência resolvida. A mecânica concreta está em `add-0001` §12.
* Nenhuma convenção adicional de release (ex: tag) é exigida do autor do plugin além do campo de versão já declarado no manifesto (ADR-0012) — mantém a promessa de baixa fricção de autoria; o custo é uma checagem incremental por plugin, aceitável por só acontecer em consultas explícitas, nunca em `start`/`resume`/`archive`.

## Alternativas consideradas

* **Totalmente manual, só individual.** Não escala além de poucos plugins — descartada como única opção (mantida como caminho disponível dentro do modelo acima).
* **Automática/silenciosa** (estilo extensão de browser). **Descartada diretamente**: um plugin poderia mudar de comportamento entre execuções sem o usuário saber, contradizendo RNF-3 e o risco de plugin malicioso/mal escrito já nomeado no PRD. Não há trade-off que justifique considerar essa opção para este produto.
* **Exigir uma tag de release correspondente à versão declarada**, para permitir checagem sem transferir nenhum objeto. Tecnicamente mais barata, mas rejeitada para v1 por adicionar uma obrigação de autoria além do que já é prometido; anotada como possível otimização futura de performance, não mudança de requisito.
* **Notificação passiva em `start`/`resume`.** Reduz o "esquecer de checar", mas exigiria um cache com TTL para não violar o espírito de RNF-4 se a checagem fosse síncrona — anotada como candidato de v2 não-bloqueante.

## Consequências

**Positivas:** sem nova obrigação ao autor além do já prometido; nunca muta a cópia de trabalho ativa de um plugin durante uma simples checagem; um único campo (`version`) é o sinal de "o que mudou" em todo o sistema.

**Negativas / trade-offs:** toda checagem de "o que está desatualizado" ainda custa uma consulta de rede por plugin — aceitável por ser iniciada pelo usuário e pouco frequente, não é um bloqueio para v1.
