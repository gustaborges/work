# ADR-0008: Assinatura/Verificação de Pacotes de Terceiros — Adiada, com Justificativa Registrada

**Status:** Aceito (decisão de adiar, com justificativa registrada)
**Data:** 2026-08-23
**Contexto de produto:** `docs-v2/prd.md` — Seção 12 (risco nomeado), RNF-7
**Governa:** `docs-v2/add/add-0001-work-system-architecture.md`, Seção 11

## Contexto

O PRD nomeia "plugin externo malicioso ou mal escrito compromete dados do usuário" como risco, e RNF-7 trata minimização de confiança como valor central. A pergunta era se uma política de assinatura/verificação de pacotes precisa ser decidida já na v1.

## Decisão

Explicitamente fora do escopo da v1 — por um motivo específico, não apenas "fica para depois": no modelo de instalação da v1 (ADR-0002), a origem de um plugin é sempre uma URL ou um caminho local escolhido explicitamente pelo próprio usuário, e a referência de conteúdo é fixada no momento da instalação. Isso já dá integridade e reprodutibilidade suficientes para essa origem especificamente escolhida pelo usuário — não há, hoje, nenhum mecanismo no produto que instale um plugin sem que o usuário tenha fornecido a origem diretamente. Sem um mecanismo assim, não existe gap adicional relevante que uma assinatura fecharia: o próprio usuário já é quem decide em que origem confiar.

Essa decisão deve ser revisitada se, no futuro, o Work vier a introduzir qualquer mecanismo que instale ou sugira um plugin sem que o usuário tenha fornecido a origem diretamente — nesse momento a decisão de confiança do usuário deixaria de ser "eu escolhi essa origem" e passaria a depender de quanto ele confia nesse mecanismo intermediário, o que abriria uma classe de risco que o simples fixar de referência de conteúdo não endereça.

## Alternativas consideradas

* **Decidir e implementar assinatura já na v1.** Rejeitada: absorveria complexidade sem fechar nenhum gap de risco real hoje, já que o modelo de instalação atual já depende inteiramente de uma origem escolhida explicitamente pelo usuário.
* **Ignorar o tema indefinidamente, sem registrar a justificativa.** Rejeitada: perderia o raciocínio já feito, forçando reavaliação do zero no futuro sem contexto do porquê a decisão foi tomada.

## Consequências

**Positivas:** a v1 não absorve complexidade que não fecha nenhum gap de risco real no modelo de instalação atual; a justificativa fica registrada para não precisar ser rederivada do zero se as premissas mudarem.

**Negativas / trade-offs:** nenhuma para a v1 — o risco que motivaria assinatura (uma origem de plugin não escolhida diretamente pelo usuário) simplesmente não existe no produto hoje.
