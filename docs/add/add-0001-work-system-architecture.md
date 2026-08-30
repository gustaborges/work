# ADD-0001: Arquitetura do Sistema Work

**Status:** Rascunho

**Data:** 2026-08-29

**Decisões que governam este documento:** ADR-0000 (execução de plugins), ADR-0002 (instalação e registro), ADR-0003 (bootstrap), ADR-0004 (colisão de Starters), ADR-0005 (atualização), ADR-0006 (runtime), ADR-0008 (assinatura), ADR-0009 (stack), ADR-0011 (resolução de convenção), ADR-0012 (modelo de componentes e manifesto) e ADR-0013 (namespaces e estado persistido).

**Requisitos atendidos:** `docs/prd.md` — RF-1 a RF-38, RNF-1 a RNF-7. Este documento implementa os requisitos e decisões listados; incompatibilidades devem ser resolvidas no PRD ou em ADR, nunca por divergência silenciosa de design.

***

## 1. Visão geral de componentes

O sistema possui três tipos de processo:

* **Binário core do Work** — interpreta comandos, exibe TUI, mantém estado local e controla o lifecycle.
* **`git`** — ferramenta invocada pelo core para worktrees e resolução de branches.
* **Processos de plugin** — subprocessos de curta duração, um por operação executável; o core nunca carrega seu código no próprio processo.

O estado vive em `~/.work/` (plugins, configuração e o índice SQLite) e no diretório de workspace escolhido pelo usuário para cada Work. O core cria apenas a worktree e `work-state.json`; qualquer outro arquivo ou diretório pode ser incorporado por Importers, sem significado especial para o core.

***

## 2. Stack concreto

Conforme ADR-0009:

* **Linguagem:** Go, em binário único e portável.
* **CLI:** Cobra.
* **TUI:** Bubble Tea, para seleções interativas.
* **Persistência:** `work-state.json` é o snapshot canônico de cada Work; SQLite embutido com driver Go puro, em `~/.work/state/work.db`, é sua projeção global de consulta.

***

## 3. Layout de diretórios e estado do núcleo

O diretório de workspace é escolhido pelo usuário no primeiro uso que exija sua materialização e guardado na configuração. O core pode sugerir um caminho padrão, mas a localização não é fixa nem faz parte da identidade de um Work. Alterações posteriores da configuração afetam novos Works e não movem automaticamente Works já existentes. A estrutura de cada Work é:

```text
<workspace>/
  in-progress/<repo-name>_<branch-name>/
    worktree/
    work-state.json
  archived/<yyyymmdd>-<repo-name>_<branch-name>/
    work-state.json
    # a worktree foi destruída; artefatos de Importers, se houver, são preservados
```

`work-state.json` é o snapshot canônico e autocontido de cada Work. Ele possui as seções semânticas `work`, `meta` e `links`, em um único arquivo físico:

```jsonc
{
  "schema": 1,
  "work": {
    "slug": "meu-trabalho",
    "status": "in-progress",
    "start_mode": "fork",
    "starter": "github-pull-request-starter",
    "branch": "feature/meu-trabalho",
    "base_branch": "main",
    "branch_convention": "gitflow",
    "created_at": "2026-08-29T00:00:00Z",
    "last_accessed_at": "2026-08-29T00:00:00Z"
  },
  "meta": {
    "github.pull_request.number": 212
  },
  "links": {
    "github.pull_request": "https://github.com/example/project/pull/212"
  }
}
```

`work` contém exclusivamente estado governado e versionado pelo core; plugins não criam nem escrevem suas propriedades. `meta` contém dados extensíveis publicados por componentes, e `links` contém relações externas de primeira classe. `branch_convention` é omitida em contribuição. O caminho do repositório resolvido pelo Starter e o checkout corrente não usam um nome persistido ambíguo; quando um componente precisar acessar o checkout, o core poderá expor a chave inequívoca `work:worktree_path` como input.

Cada chave de link possui um único valor corrente e toda publicação é upsert: a última origem vence. A proveniência operacional (`source_component`, `source_operation`, `recorded_at`) é mantida pelo core no índice, separada da chave e do valor semântico. O arquivo é atualizado por gravação temporária e rename atômico. `work.db` é uma projeção/indexação para consultas globais, como listagem e ordenação; ele pode ser reconstruído ou reconciliado a partir dos snapshots e não é a única fonte do estado de um Work.

***

## 4. Pacote de plugin e manifesto

`plugin.json`, na raiz do pacote, descreve estaticamente componentes executáveis e convenções declarativas:

```jsonc
{
  "name": "github-plugin",
  "version": "1.2.0",
  "components": [
    {
      "name": "github-pull-request-starter",
      "role": "starter",
      "pattern": "^https://github\\.com/[^/]+/[^/]+/pull/\\d+$",
      "entrypoint": "starter.py",
      "runtime": "python3"
    },
    {
      "name": "github-pull-request-importer",
      "role": "importer",
      "on": [{
        "event": "start:finalized",
        "starters": ["github-pull-request-starter"]
      }],
      "manual": {
        "display_name": "Contexto do Pull Request",
        "description": "Importa os artefatos associados ao pull request"
      },
      "inputs": ["link:github.pull_request", "work:start_mode:optional"],
      "entrypoint": "importer.py",
      "runtime": "python3"
    },
    {
      "name": "github-pull-request-linker",
      "role": "linker",
      "key": "github.pull_request",
      "discover": {
        "automatic": true,
        "on": [{
          "event": "start:finalized",
          "starters": ["github-pull-request-starter"]
        }],
        "inputs": ["work:worktree_path"]
      },
      "manual": {
        "display_name": "Pull Request do GitHub",
        "description": "Relaciona o Work a um pull request do GitHub"
      },
      "entrypoint": "linker.py",
      "runtime": "python3"
    }
  ],
  "conventions": [
    {
      "name": "gitflow",
      "prefixes": ["feature/{slug}", "release/{slug}", "hotfix/{slug}"]
    }
  ]
}
```

O pacote é a unidade atômica de instalação, atualização, habilitação e remoção. A identidade de componente e convenção é o `name` lógico; o nome do pacote é um alias local proposto. Nomes de componente ambíguos entre pacotes habilitados são qualificados como `<alias-do-pacote>/<nome>`.

### 4.1 Validação por role

`role` é obrigatório e é o discriminador semântico do componente.

| Role | Obrigatório | Permitido | Inválido |
| --- | --- | --- | --- |
| `starter` | `name`, `role`, `entrypoint` | `pattern`, `runtime` | `on`, `manual`, `inputs`, `key`, `discover` |
| `importer` | `name`, `role`, `entrypoint`, ao menos um de `on` ou `manual` | `on`, `manual`, `inputs`, `runtime` | `pattern`, `key`, `discover` |
| `linker` | `name`, `role`, `key`, `entrypoint`, ao menos um de `discover` ou `manual` | `discover`, `manual`, `runtime` | `pattern` |

`conventions[]` permanece fora de `components[]`; sua entrada tem somente `name` e `prefixes[]`. Campos incompatíveis tornam a instalação ou atualização inválida. Não existe campo `invocation`, `type`, `capabilities` ou `hooks`.

### 4.2 Namespaces de dados e Semantic Conventions

`inputs[]` usa a gramática `<source>:<key>[:optional]`, onde `source` é `work`, `meta` ou `link`. `work:<key>` acessa somente a superfície de estado exposta pelo core; `meta:<key>` acessa metadata extensível; e `link:<key>` acessa uma relação externa de primeira classe. O namespace é usado para resolução e elegibilidade, mas não é repetido no payload projetado.

Chaves públicas de `meta` e `link` seguem as Semantic Conventions do Work: definem namespace, chave, significado e representação do valor, sem compor uma whitelist fechada no binário. Dados sem semântica compartilhada usam namespace privado `plugin.<nome-do-plugin>.<key>`. A chave identifica o significado do dado, não seu produtor: múltiplos Linkers podem declarar a mesma `key` sem criar colisão de namespace.

***

## 5. Instalação e registro

```text
~/.work/
  plugins/<alias>/
    plugin.json
    source/
    .install-meta.json
  config/
    work.json
  state/
    work.db
```

`work plugin install <origem>` clona fonte remota e fixa uma referência; `work plugin install --local <path>` cria link para desenvolvimento local. O registro é gerado do manifesto e armazena, por componente, alias do pacote, nome, role, entrypoint, runtime, pattern, eventos e filtros, inputs, chave de Linker, descoberta e apresentação manual. A instalação valida a gramática e os namespaces de `inputs[]` e a sintaxe das chaves declaradas no manifesto; dados publicados em outputs são validados quando recebidos. Convenções são registradas separadamente com nome e prefixos.

Conflito de alias de pacote de origem diferente falha; `--as <alias>` resolve-o. Reinstalação da mesma origem sob o mesmo alias é idempotente. A alteração do modelo de campos é validada pelo mesmo pipeline de instalação e atualização.

***

## 6. Bootstrap do pacote de referência

O seed oficial contém, no mínimo, um Starter fallback para repositório local e a convenção `freeform` com prefixo `{slug}`. No primeiro `work init` ou `work start`, ele passa pelo pipeline normal de instalação, sem rede. Pode ser desinstalado como qualquer outro pacote.

***

## 7. Resolução de Starters e criação do Work

Em `work start <arg>`, Starters específicos (`pattern` presente e não vazio) são avaliados localmente contra o argumento. Um único match é usado; múltiplos matches são escolhidos por TUI; ausência de match usa o único fallback habilitado. A colisão é perguntada a cada ocorrência. A ausência de fallback gera erro acionável que orienta habilitar ou instalar um Starter.

O Starter recebe:

```json
{ "arg": "https://github.com/example/project/pull/212" }
```

Com sucesso, devolve os dados que conseguiu resolver. Campos de núcleo são explícitos; dados de integração entram em `meta` e relações externas em `links`:

```jsonc
{
  "repo_path": "/home/user/projects/project",
  "base_branch": "feature/source-branch",
  "start_modes": ["contribution", "fork"],
  "meta": { "github.pull_request.number": 212 },
  "links": {
    "github.pull_request": "https://github.com/example/project/pull/212"
  }
}
```

Campos opcionais aparecem apenas quando resolvidos. Ausência de `start_modes` significa Work novo. Quando presente, o core oferece exatamente os modos retornados e persiste o modo escolhido em `work.start_mode`; para ausência, persiste `new`. `repo_path` pertence somente ao contrato de resolução do Starter: ele identifica o repositório de origem para o fluxo de criação e não é promovido automaticamente a metadata ou a input público. A convenção de branch não é entrada nem saída do Starter. Falha de subprocesso do Starter escolhido falha `work start`; não há resultado `matched: false` ou equivalente.

Antes de continuar o fluxo de criação, o core valida que `repo_path` existe, é acessível e identifica um repositório Git local. Falha nessa validação encerra `work start` antes de qualquer materialização do Work, com diagnóstico que identifica o caminho rejeitado. Essa validação pertence ao core porque Git e a criação da worktree fazem parte do lifecycle governado por ele. Uma vez escolhido estaticamente pelo `pattern`, falha do Starter ou resposta estruturalmente inválida continua sendo falha do comando.

***

## 8. Convenção de branch

Cada pacote habilitado contribui seu `conventions[]` para o catálogo global. Fora de contribuição, o Work calcula a identidade do repositório, reutiliza a convenção memorizada em `repo_branch_convention` ou pede uma escolha e a persiste. Depois apresenta seus prefixos. `work convention` exibe a escolha do repositório atual; `work convention set` a substitui. A identidade usa, nesta ordem, URL de `origin`, commits raiz ou caminho absoluto em clone raso sem remote.

Fora do modo contribuição, depois de interpolar o prefixo da convenção com o slug escolhido, o core valida o nome de branch resultante utilizando as regras do próprio Git. Também verifica colisões com branches locais e remotas já existentes antes de materializar a nova branch/worktree. Nome inválido ou colisão impede a criação e retorna o fluxo à escolha que produziu o nome.

O core deve preferir as operações nativas do Git para essas verificações, incluindo `git check-ref-format --branch` para validade sintática e consultas de refs para detectar nomes já existentes, em vez de manter uma implementação paralela das regras de nomes de branch. No modo contribuição essa regra não exige que a branch seja nova, pois o comportamento esperado é justamente fazer checkout da branch resolvida pelo Starter.

***

## 9. Eventos, elegibilidade e Linkers

Eventos são identificadores de lifecycle definidos pelo core no formato `<comando>:<evento>`. Plugins se inscrevem, mas não criam eventos. Na v1, `start:finalized` ocorre depois da materialização do Work e de seu estado de núcleo.

`on[]` de Importer e `discover.on[]` de Linker aceitam `event` e filtro opcional `starters`; sem filtro, a inscrição vale para qualquer Starter. O filtro decide elegibilidade e nunca é entregue ao subprocesso.

Antes de abrir um subprocesso, o core verifica: plugin habilitado; operação disponível para a ativação corrente; inscrição e filtro quando a ativação é por evento; e existência de todos os inputs obrigatórios. Inputs usam `work:<key>`, `link:<key>` ou `meta:<key>`, com `:optional` para ausência aceitável. Inputs opcionais não participam da elegibilidade e são omitidos da entrada se ausentes. O core resolve o namespace, mas projeta no payload somente a chave solicitada e seu valor.

Um Linker declara uma `key` namespaced. Vários Linkers podem produzir a mesma chave sem que isso seja colisão: são providers alternativos da mesma relação semântica. `discover.automatic` permite descoberta solicitada pelo Work; `discover.on[]` permite descoberta por evento. A descoberta recebe apenas seus inputs e devolve opcionalmente `{ "value": "..." }`; sucesso sem valor não altera o estado e não é erro. Com valor, o core faz upsert em `links[key]` e registra sua proveniência no índice. `manual` torna o Linker disponível em `work link`: após seleção, o core coleta um valor não vazio e faz o mesmo upsert, sem iniciar subprocesso.

***

## 10. Importers e staging

Um Importer possui a única operação de domínio `import`, ativável por `on`, `manual` ou ambos. `manual` o torna disponível em `work import` no Work atual. A elegibilidade e os inputs são idênticos entre ativação manual e por evento.

O core cria um diretório temporário exclusivo para cada execução e envia apenas os inputs resolvidos e `output_dir`:

```jsonc
{
  "inputs": {
    "github.pull_request": "https://github.com/example/project/pull/212",
    "start_mode": "contribution"
  },
  "output_dir": "/tmp/work/import-01J..."
}
```

Após sucesso, o core enumera todo conteúdo produzido, calcula destinos no diretório do Work e verifica todas as colisões antes de modificá-lo. Sem colisões, incorpora os artefatos e remove o temporário. Com colisão ou falha, não incorpora nenhum artefato daquela execução e remove o temporário. O staging é fronteira de dados, não sandbox.

***

## 11. Ordem de `start:finalized` e contrato de execução

Ao publicar `start:finalized`, o core executa as fases:

```text
Starter resolvido
  → estado `work`, links e metadata do Starter persistidos atomicamente
  → Linkers inscritos e elegíveis
  → novos links persistidos
  → Importers inscritos e elegíveis
  → staging, validação de colisões e incorporação
```

Não há ordem garantida entre componentes do mesmo role dentro de uma fase. A elegibilidade dos Importers é avaliada após os Linkers, permitindo consumir links recém-descobertos.

Todo componente executável recebe exatamente um JSON por stdin, escreve no máximo um JSON estruturado por stdout quando sua operação o exige e usa código de saída para sucesso ou falha. Com `runtime`, o comando é `<runtime> <entrypoint>`; sem ele, o entrypoint é executável autocontido. O runtime é checado na instalação. O core não depende de estado implícito entre invocações e nunca executa componente durante descoberta ou instalação para auto-descrição.

Na v1, o protocolo IPC não possui mensagens intermediárias de progresso, heartbeat ou streaming de status. Durante uma execução, o core pode apresentar um indicador visual contendo o componente e a operação corrente, mas o subprocesso produz somente o resultado final previsto pelo contrato.

O core não impõe timeout próprio às execuções de Starter, Importer ou Linker na v1. Cancelamento explícito pelo usuário ou encerramento do processo continuam podendo interromper a operação. Políticas específicas de retry, latência ou indisponibilidade de serviços externos pertencem ao componente.

Falhas de Linker ou Importer automáticos são registradas e exibidas como aviso, mas não desfazem o Work já criado. Em comandos manuais, a falha é retornada ao usuário sem alteração de links ou incorporação de artefatos. Plugins nunca leem ou escrevem `work-state.json` diretamente: recebem inputs e retornam outputs por contratos IPC; o core converte esses contratos para o snapshot persistido.

***

## 12. Atualização, TUI e integridade

`work view` é uma operação somente leitura sobre o Work atual. O core lê o snapshot canônico e apresenta os links persistidos em `links`; não executa Linkers, não dispara descoberta e não atualiza proveniência ou timestamps de links. Fora de um diretório associado a um Work, o comando falha com mensagem acionável.

A apresentação utiliza a chave semântica e o valor persistido. Comportamentos adicionais específicos de representação — por exemplo, oferecer navegação quando uma Semantic Convention definir um valor navegável — podem ser acrescentados sem alterar a semântica básica do comando.

`work plugin outdated` consulta atualizações sem mudar checkout; `work plugin update <nome>` e `--all` alteram versões somente mediante ação explícita. A atualização lê e valida o novo manifesto antes de trocar a versão registrada.

`work plugin` oferece TUI sobre os mesmos comandos diretos de gestão. `work import` e `work link` usam a TUI para listar somente componentes manuais disponíveis e elegíveis no Work atual. A origem de todos os plugins é explicitamente escolhida pelo usuário e a referência instalada é fixada; assinatura formal fica adiada conforme ADR-0008.
