# ADD-0001: Arquitetura do Sistema Work

**Status:** Rascunho

**Data:** 2026-08-29

**Decisões que governam este documento:** ADR-0000 (execução de plugins), ADR-0002 (instalação e registro), ADR-0003 (bootstrap), ADR-0004 (colisão de Starters), ADR-0005 (atualização), ADR-0006 (runtime), ADR-0008 (assinatura), ADR-0009 (stack), ADR-0011 (resolução de convenção) e ADR-0012 (modelo de componentes e manifesto).

**Requisitos atendidos:** `docs/prd.md` — RF-1 a RF-32, RNF-1 a RNF-7. Este documento implementa os requisitos e decisões listados; incompatibilidades devem ser resolvidas no PRD ou em ADR, nunca por divergência silenciosa de design.

***

## 1. Visão geral de componentes

O sistema possui três tipos de processo:

* **Binário core do Work** — interpreta comandos, exibe TUI, mantém estado local e controla o lifecycle.
* **`git`** — ferramenta invocada pelo core para worktrees e resolução de branches.
* **Processos de plugin** — subprocessos de curta duração, um por operação executável; o core nunca carrega seu código no próprio processo.

O estado vive em `~/.work/` (plugins, configuração e SQLite) e no diretório de workspace escolhido pelo usuário para cada Work. O core cria apenas a worktree e `work-meta.json`; qualquer outro arquivo ou diretório pode ser incorporado por Importers, sem significado especial para o core.

***

## 2. Stack concreto

Conforme ADR-0009:

* **Linguagem:** Go, em binário único e portável.
* **CLI:** Cobra.
* **TUI:** Bubble Tea, para seleções interativas.
* **Persistência:** SQLite embutido com driver Go puro, em `~/.work/state/work.db`.

***

## 3. Layout de diretórios e estado do núcleo

O diretório de workspace é escolhido pelo usuário no primeiro uso e guardado na configuração. A estrutura de cada Work é:

```text
<workspace>/
  in-progress/<repo-name>_<branch-name>/
    worktree/
    work-meta.json
  archived/<yyyymmdd>-<repo-name>_<branch-name>/
    work-meta.json
    # a worktree foi destruída; artefatos de Importers, se houver, são preservados
```

`work-meta.json` mantém os campos de núcleo, a metadata publicada e os links persistidos:

```jsonc
{
  "slug": "meu-trabalho",
  "branch": "feature/meu-trabalho",
  "base_branch": "main",
  "starter": "github-pull-request-starter",
  "branch_convention": "gitflow",
  "meta": {
    "repo_path": "/home/user/projects/project",
    "start_mode": "contribution",
    "github.pull_request_number": 212
  },
  "links": {
    "github.pull_request": "https://github.com/example/project/pull/212"
  },
  "created_at": "2026-08-29T00:00:00Z",
  "last_accessed_at": "2026-08-29T00:00:00Z",
  "status": "in-progress"
}
```

`branch_convention` é omitida em contribuição. O core materializa em `meta` os dados conhecidos do Work, incluindo `repo_path` e `start_mode` (`new`, `contribution` ou `fork`), e incorpora também a metadata publicada pelo Starter. A tabela `works` espelha os campos necessários para listagem e status. Tabelas ou colunas auxiliares para metadata e links mantêm o mesmo conteúdo do arquivo e são atualizadas transacionalmente; cada chave de link possui um único valor e toda publicação é upsert, portanto a última origem vence.

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
      "inputs": ["link:github.pull_request", "meta:start_mode:optional"],
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
        "inputs": ["meta:repo_path"]
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

`work plugin install <origem>` clona fonte remota e fixa uma referência; `work plugin install --local <path>` cria link para desenvolvimento local. O registro é gerado do manifesto e armazena, por componente, alias do pacote, nome, role, entrypoint, runtime, pattern, eventos e filtros, inputs, chave de Linker, descoberta e apresentação manual. Convenções são registradas separadamente com nome e prefixos.

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
  "meta": { "github.pull_request_number": 212 },
  "links": [{
    "key": "github.pull_request",
    "value": "https://github.com/example/project/pull/212"
  }]
}
```

Campos opcionais aparecem apenas quando resolvidos. Ausência de `start_modes` significa Work novo. Quando presente, o core oferece exatamente os modos retornados e persiste o modo escolhido em `meta.start_mode`; para ausência, persiste `new`. O core também persiste `repo_path` em `meta.repo_path`, para que ambos possam ser solicitados como inputs. A convenção de branch não é entrada nem saída do Starter. Falha de subprocesso do Starter escolhido falha `work start`; não há resultado `matched: false` ou equivalente.

***

## 8. Convenção de branch

Cada pacote habilitado contribui seu `conventions[]` para o catálogo global. Fora de contribuição, o Work calcula a identidade do repositório, reutiliza a convenção memorizada em `repo_branch_convention` ou pede uma escolha e a persiste. Depois apresenta seus prefixos. `work convention` exibe a escolha do repositório atual; `work convention set` a substitui. A identidade usa, nesta ordem, URL de `origin`, commits raiz ou caminho absoluto em clone raso sem remote.

***

## 9. Eventos, elegibilidade e Linkers

Eventos são identificadores de lifecycle definidos pelo core no formato `<comando>:<evento>`. Plugins se inscrevem, mas não criam eventos. Na v1, `start:finalized` ocorre depois da materialização do Work e de seus metadados de núcleo.

`on[]` de Importer e `discover.on[]` de Linker aceitam `event` e filtro opcional `starters`; sem filtro, a inscrição vale para qualquer Starter. O filtro decide elegibilidade e nunca é entregue ao subprocesso.

Antes de abrir um subprocesso, o core verifica: plugin habilitado; operação disponível para a ativação corrente; inscrição e filtro quando a ativação é por evento; e existência de todos os inputs obrigatórios. Inputs usam `link:<key>` ou `meta:<key>`, com `:optional` para ausência aceitável. Inputs opcionais não participam da elegibilidade e são omitidos da entrada se ausentes.

Um Linker é responsável por `key` namespaced. `discover.automatic` permite descoberta solicitada pelo Work; `discover.on[]` permite descoberta por evento. A descoberta recebe apenas seus inputs e devolve opcionalmente `{ "value": "..." }`; sucesso sem valor não altera o estado e não é erro. Com valor, o core faz upsert em `links[key]`. `manual` torna o Linker disponível em `work link`: após seleção, o core coleta um valor não vazio e faz o mesmo upsert, sem iniciar subprocesso.

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
  → links e metadata do Starter persistidos
  → Linkers inscritos e elegíveis
  → novos links persistidos
  → Importers inscritos e elegíveis
  → staging, validação de colisões e incorporação
```

Não há ordem garantida entre componentes do mesmo role dentro de uma fase. A elegibilidade dos Importers é avaliada após os Linkers, permitindo consumir links recém-descobertos.

Todo componente executável recebe exatamente um JSON por stdin, escreve no máximo um JSON estruturado por stdout quando sua operação o exige e usa código de saída para sucesso ou falha. Com `runtime`, o comando é `<runtime> <entrypoint>`; sem ele, o entrypoint é executável autocontido. O runtime é checado na instalação. O core não depende de estado implícito entre invocações e nunca executa componente durante descoberta ou instalação para auto-descrição.

Falhas de Linker ou Importer automáticos são registradas e exibidas como aviso, mas não desfazem o Work já criado. Em comandos manuais, a falha é retornada ao usuário sem alteração de links ou incorporação de artefatos.

***

## 12. Atualização, TUI e integridade

`work plugin outdated` consulta atualizações sem mudar checkout; `work plugin update <nome>` e `--all` alteram versões somente mediante ação explícita. A atualização lê e valida o novo manifesto antes de trocar a versão registrada.

`work plugin` oferece TUI sobre os mesmos comandos diretos de gestão. `work import` e `work link` usam a TUI para listar somente componentes manuais disponíveis e elegíveis no Work atual. A origem de todos os plugins é explicitamente escolhida pelo usuário e a referência instalada é fixada; assinatura formal fica adiada conforme ADR-0008.
