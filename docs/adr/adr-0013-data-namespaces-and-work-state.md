# ADR-0013: Namespaces de Dados e Estado Persistido do Work

**Status:** Aceito

**Data:** 2026-08-29

**Contexto de produto:** `docs/prd.md` — RF-7, RF-26, RF-27, RF-32 a RF-35 e RNF-5

**Governa:** `docs/add/add-0001-work-system-architecture.md`, Seções 3, 4, 7, 9 e 11

## Contexto

O modelo de componentes precisa transportar tanto estado de domínio do Work quanto dados extensíveis publicados por plugins e relações externas. Se esses dados compartilharem um único espaço semântico, plugins podem sobrescrever estado do núcleo, relações externas ficam indistintas de metadata e a mesma informação tende a ser nomeada pela ferramenta que a produziu, impedindo interoperabilidade entre providers.

Também é necessário manter o estado de cada Work auditável e preservável após o arquivamento, sem tornar a estrutura física de persistência uma API de plugins. Um banco global é útil para listagem e busca, mas não deve ser a única fonte necessária para compreender ou recuperar um Work individual.

## Decisão

O Work reconhece três namespaces de input: `work:<key>`, `meta:<key>` e `link:<key>`. `work` contém somente estado de domínio definido e governado pelo core; plugins não podem criar ou escrever chaves nesse namespace. `meta` contém informação extensível produzida por componentes. `link` contém relações externas de primeira classe, com no máximo um valor corrente por chave e upsert de última origem vencedora.

Chaves públicas de `meta` e `link` seguem Semantic Conventions do Work, que definem namespace, chave, significado e representação do valor sem formar uma enumeração fechada no binário. Dados que não sejam contratos de interoperabilidade usam namespace privado `plugin.<nome-do-plugin>.<key>`. A chave identifica o significado do dado, não seu provider: componentes distintos podem produzir a mesma chave, e a proveniência concreta é registrada separadamente pelo core.

Cada Work possui `work-state.json` como snapshot canônico, autocontido e versionado por `schema`, com as seções `work`, `meta` e `links`. O core atualiza o arquivo por escrita temporária seguida de rename atômico. Plugins não leem nem escrevem esse arquivo; interagem somente por contratos de `inputs` e `outputs`.

`~/.work/state/work.db` é uma projeção/indexação global para consultas do CLI. Pode conter campos normalizados e proveniência operacional, mas deve poder ser reconstruído ou reconciliado a partir dos snapshots. Não há requisito de identidade estrutural entre os payloads IPC e o arquivo de estado: o core traduz respostas de operações para o estado resultante.

Esta ADR substitui as partes de ADR-0012 que limitavam `inputs[]` a `link` e `meta` e tratavam a persistência de links/metadata sem o modelo explícito de estado canônico definido aqui.

## Alternativas consideradas

* **Um único mapa de metadata para todos os dados.** Rejeitada: permite que dados de plugin se confundam com estado de domínio e elimina o tratamento genérico de relações externas necessário para elegibilidade, Linkers e operações de link.
* **Incluir o provider na chave semântica.** Rejeitada: `github.pull_request.via_api` e `github.pull_request.via_gh_cli` descrevem a mesma relação e fragmentariam produtores e consumidores. Provider e proveniência pertencem ao registro operacional, não à identidade do dado.
* **Usar `work.db` como única fonte de verdade.** Rejeitada: impede que o diretório de um Work arquivado seja autocontido e torna recuperação/auditoria dependente de uma base global separada.
* **Expor `work-state.json` diretamente a plugins.** Rejeitada: transforma layout físico em API pública, acopla plugins a migrações de schema e permite escrita fora do lifecycle controlado pelo core.
* **Um arquivo por domínio (`work.json`, `meta.json`, `links.json`).** Rejeitada: uma transição pode afetar os três domínios; um snapshot único permite validar e publicar a atualização como uma unidade atômica.

## Consequências

**Positivas:** ownership e elegibilidade tornam-se explícitos; plugins interoperam por significado, não por implementação; estado de cada Work permanece auditável e recuperável; o índice global pode evoluir sem ser uma dependência de integridade do Work.

**Trade-offs:** o core precisa validar namespaces, chaves e schema, manter a projeção reconciliável e definir cuidadosamente quais propriedades `work:*` são expostas. A persistência de histórico completo de proveniência no diretório do Work continua fora do escopo; o índice pode registrar apenas a proveniência operacional necessária.
