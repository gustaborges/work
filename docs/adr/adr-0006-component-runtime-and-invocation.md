# ADR-0006: Tecnologia e Modelo de Invocação de Componentes

**Status:** Aceito
**Data:** 2026-08-23
**Contexto de produto:** `docs/prd.md` — RNF-2, RNF-6
**Governa:** `docs/add/add-0001-work-system-architecture.md`, Seção 11

## Contexto

O produto promete que qualquer pessoa deve poder escrever um plugin sem depender de um merge no repositório do Work (RNF-2); RNF-6 exige portabilidade entre os sistemas operacionais suportados. O modelo de execução por processo externo (ADR-0000) já é o que torna a neutralidade de linguagem possível — falta decidir como o núcleo efetivamente invoca um processo escrito em uma linguagem arbitrária de forma portátil.

## Decisão

* Nenhuma restrição de linguagem/stack é imposta a um plugin. Restringir a binários autocontidos foi considerado e rejeitado: não compra segurança nenhuma (um binário malicioso é tão perigoso quanto um script malicioso — restringir stack é controle de portabilidade, não de segurança) e excluiria o autor mais provável de um plugin simples, que prefere uma linguagem de script a aprender uma nova linguagem só para publicar uma integração pequena.
* O manifesto (ADR-0001) declara opcionalmente qual interpretador um componente precisa (ex: `python3`, `node`, `sh`; ausente significa executável autocontido).
* O Work **nunca** depende de shebang ou do bit de execução do sistema operacional para escolher o interpretador — invoca explicitamente o interpretador declarado sobre o entrypoint quando ele existe, ou o entrypoint diretamente quando ausente. Isso é um requisito de portabilidade, não estilo: linhas de shebang não são interpretadas pela criação de processo do Windows — depender delas quebraria silenciosamente todo plugin em linguagem interpretada (provável maioria do ecossistema) em um SO suportado (RNF-6).
* A presença do interpretador declarado é checada em preflight (na instalação/antes da primeira invocação): se ele não existe no `PATH`, o Work falha com uma mensagem específica e acionável, em vez de deixar o subprocesso falhar de forma críptica.
* Um modelo de detecção automática entre binário pré-compilado por plataforma e script interpretado via shebang (visto em outras ferramentas de extensão de CLI) foi avaliado e não adotado tal qual: ele resolve um problema diferente (escolher entre múltiplos *artefatos* de release para a mesma extensão lógica, por plataforma), e seu modo interpretado ainda depende de shebang + bit de execução — exatamente o mecanismo evitado aqui por causa do Windows.

## Alternativas consideradas

* **Totalmente aberto, sem declaração de interpretador no manifesto.** Fricção mínima ao autor, mas produz falhas do tipo "formato de executável inválido" sem contexto quando falta o interpretador — rejeitada.
* **Restringir a binários autocontidos.** Elimina a classe de erro "faltou o interpretador", mas contradiz RNF-2/ADR-0000 e não melhora segurança, como descrito acima — rejeitada.

## Consequências

**Positivas:** plugins em qualquer linguagem funcionam de forma uniforme entre os SOs suportados, sem depender de mecanismo de SO não-portável; falhas por interpretador ausente são diagnosticadas claramente na instalação/preflight, não em uma invocação arbitrária depois.

**Negativas / trade-offs:** o interpretador declarado é autodeclarado, sem verificação independente além da checagem de `PATH` no preflight (mesma ressalva já existente para capabilities autodeclaradas) — mitigado pelo mesmo mecanismo: a auto-descrição (ADR-0001) é a checagem prática de que o componente de fato roda.
