# ADD-0001: Arquitetura do Sistema Work

**Status:** Rascunho
**Data:** 2026-08-23

**Decisões que governam este documento:** ADR-0000 (modelo de execução de plugins), ADR-0001 (manifesto e pacotes), ADR-0002 (instalação e registro), ADR-0003 (bootstrap do pacote de referência), ADR-0004 (prioridade e colisão de starters), ADR-0005 (atualização de plugins), ADR-0006 (runtime e invocação de componentes), ADR-0007 (TUI de gestão de plugins), ADR-0008 (assinatura adiada), ADR-0009 (stack central).

**Requisitos atendidos (referência, não reinterpretação):** `docs/prd.md` — RF-1 a RF-22, RNF-1 a RNF-7. Este documento descreve como a arquitetura é estruturada para satisfazer esses requisitos; não redefine nem reinterpreta o que eles pedem. Uma incompatibilidade encontrada aqui é motivo para revisar o PRD ou uma ADR, nunca para o design divergir silenciosamente.

***

## 1. Visão geral de componentes

O sistema tem três tipos de processo em execução:

* **Binário core do Work** — o único processo de longa duração, que interpreta comandos, roda a TUI e mantém o estado local.
* **`git`** — a única ferramenta invocada diretamente/in-process pelo core (ADR-0000), para criar/destruir worktrees e resolver branches remotas/locais.
* Processos de plugin — subprocessos de curta duração, um por invocação, comunicando com o core por um contrato de entrada/saída bem definido (Seção 9). O core nunca carrega código de plugin no próprio processo.

O estado do sistema vive inteiramente em disco, sob `~/.work/` (plugins, config, registro) e :comment[\~/workspace/]{#comment-1787524842428 text="isso é convenção da minha máquina; o cli deve, antes de definir o diretorio do workspace, sugeri-lo ao usuário; pode ser que esse diretorio já esteja em uso pelo usuario, o usuario deve ter a oportunidade de especificar o diretório onde ele quer trabalhar; isso nos leva a pensar na experiencia de primeiro uso do pelo usuário. "} (worktrees e contexto de cada Work) — ver Seções 3 e 5.

***

## 2. Stack concreto

Per ADR-0009:

* **Linguagem:** Go — binário único, portável, sem runtime externo.
* **CLI:** Cobra.
* **TUI:** Bubble Tea, para todas as telas interativas de seleção (plugins concorrentes, prefixos de branch, base branch, `work resume`, `work archive`, gestão de plugins).
* **Persistência de estado:** SQLite embutido, via driver Go puro (sem CGO), em `~/.work/state/work.db`.

***

## 3.:comment[Layout de diretórios e estado do núcleo]{#comment-1787525101294 text="a pasta contexto e review devem ser removidas deste documento como criações do work cli. APenas worktree e work-meta.json são outputs do CLI; importers externos podem se somar a isso, acrescentando diretorios e arquivos de acordo com suas necessidades, que não importam ao work-cli. "}

```
~/workspace/
  in-progress/<repo-name>_<branch-name>/
    worktree/
    context/
    work-meta.json
  reviews/
  archived/<yyyymmdd>_<repo-name>_<branch-name>/
    context/
    work-meta.json
    # worktree é destruída na arquivamento; contexto e metadados são preservados
```

`work-meta.json`, criado dentro de cada pasta de Work:

```json
{
  "slug": "meu-trabalho",
  "branch": "feature/meu-trabalho",
  "base_branch": "main",
  "starter": "github-pull-request-starter",
  "created_at": "2026-08-23T00:00:00Z",
  "last_accessed_at": "2026-08-23T00:00:00Z",
  "status": "in-progress"
}
```

Tabela `works` (SQLite, `~/.work/state/work.db`), espelhando `work-meta.json` de cada trabalho para permitir listagem rápida — ordenada por `last_accessed_at` — sem varrer o filesystem (RF-11). `last_accessed_at` é atualizado toda vez que `work resume` seleciona aquele trabalho.

`reviews/` existe no layout mas hoje não é populada por nenhum fluxo definido — ver PRD, Apêndice A ("Escopo exato de um estado em revisão"), item de produto ainda em aberto.

***

## 4. Pacote de plugin e manifesto (ADR-0001)

`plugin.json`, na raiz do pacote:

```json
{
  "name": "github-pull-request-plugin",
  "version": "1.2.0",
  "components": [
    {
      "type": "starter",
      "name": "github-pull-request-starter",
      "pattern": "^https://github\\.com/[^/]+/[^/]+/pull/\\d+$",
      "capabilities": [
        "generate_work_slug",
        "provide-base-branch",
        "disambiguate_contribution_fork"
      ],
      "entrypoint": "starter.py",
      "runtime": "python3",
      "hooks": {
        "onFinalize": ["importers.pr-metadata-importer"]
      }
    },
    {
      "type": "importer",
      "name": "pr-metadata-importer",
      "entrypoint": "importer.py",
      "runtime": "python3"
    }
  ]
}
```

Campos do schema v1 (deliberadamente mínimo — schemas de payload por tipo de componente são definidos incrementalmente):

* `name`, `version` — identidade e versão do pacote (ver ADR-0005 para o papel de `version` na atualização).
* `components[]` — um ou mais componentes, cada um com: `type` (papel: `starter`/`importer`/`linker`/`branch-strategy`), `name` (identidade lógica, nunca o nome do arquivo), `pattern?` (regex de reconhecimento — ausente/vazio \= fallback universal, ver Seção 7), `capabilities?` (lista aberta de strings — hoje o Work reconhece `generate_work_slug`, `provide-base-branch`, `disambiguate_contribution_fork`; capability desconhecida é ignorada, permitindo adicionar novas sem quebrar plugins existentes), `entrypoint`, `runtime?` (Seção 9), `hooks?` (Seção 9).

**Identidade e alias.** A identidade de um componente é o `name` lógico do manifesto. O `name` do pacote é uma proposta de alias local, não uma garantia de unicidade — unicidade de fato é aplicada no registro (Seção 5). Referenciar um componente nos comandos usa o nome nu quando não ambíguo entre componentes habilitados; em colisão entre pacotes diferentes, a referência é qualificada como `<alias-do-pacote>/<nome-do-componente>` — reaproveita o mesmo mental model de `owner/repo` já usado na instalação, em vez de introduzir um separador novo. O `type` do componente não entra na sintaxe de referência: é metadado visível no manifesto/registro/TUI, não parte da identidade.

***

## 5. Instalação e registro (ADR-0002)

```
~/.work/
  plugins/<nome>/
    plugin.json          # manifesto do plugin — não editado pelo usuário
    source/               # conteúdo clonado, OU symlink em modo local
    .install-meta.json    # proveniência: origin_url, source_type, pinned_ref, installed_at
  config/
    work.json
    work.conventions.json
  state/
    work.db
```

`work plugin install <origem>` clona a origem para `plugins/<nome>/source/`, fixando uma referência (`source_type: git`). `work plugin install --local <path>` / `work plugin link <path>` cria um symlink para o caminho local, sem copiar (`source_type: local-link`), para desenvolvimento ativo.

Tabelas geradas em `work.db` (nunca editadas à mão, escritas por `install`/`update`/`enable`/`disable`):

* `plugins(name, source_url, source_type, pinned_ref, version, installed_at, enabled)`
* `plugin_components(plugin_name, component_type, component_name, pattern, capabilities[], entrypoint, tier)`

**Fluxo de colisão de alias.** Se o alias resultante do `name` do manifesto já existe no registro apontando para uma `source_url` diferente, `install` falha explicitamente. O usuário resolve com `work plugin install <origem> --as <alias>`, escolhendo o alias local. Reinstalar a mesma `source_url` sob o mesmo alias já existente é idempotente.

`work plugin list` e `work plugin uninstall <nome>` são comandos diretos, sem dependência de TUI.

***

## 6. Bootstrap do pacote de referência (ADR-0003)

Um pacote oficial de referência (contendo, no mínimo, um plugin `default-starter` capaz de localizar um repositório local a partir de texto livre) é vendorizado nos assets do binário/release como *seed*. No primeiro uso (`work init` ou primeiro `work start`), ele é "instalado" através do pipeline normal da Seção 5 — grava manifesto, entrada no registro, `.install-meta.json` — nunca como atalho que pule esse pipeline. Sem rede disponível na primeira execução, o seed embutido evita qualquer dependência externa; se mesmo assim a instalação falhar, o Work instrui instalação manual em vez de falhar silenciosamente. `work plugin uninstall <alias-do-pacote-de-referência>` remove esse pacote como qualquer outro.

***

## 7. Resolução de starters: camadas e colisão (ADR-0004)

Em `work start <arg>`:

* **Camada `fallback`** — componentes sem `pattern` (ou `pattern` vazio). Só um pode estar `enabled` por vez; um segundo é erro detectado em `work plugin enable`, não em runtime.
* **Camada `specific`** — componentes com `pattern`. Todo padrão `specific` habilitado é avaliado localmente contra o argumento (regex, sem subprocesso):
  * exatamente um match → invoca direto;
  * zero matches → cai no único fallback habilitado;
  * mais de um match → colisão.

**Colisão.** Reaproveita o mesmo componente de TUI de desambiguação contribuição-vs-fork para perguntar ao usuário qual componente usar. A escolha é memorizada com **chave \= o conjunto exato de componentes que colidiram** naquele match (ex: `{github-issue-starter, generic-tracker-starter}`) — nunca um padrão ou glob inferido dos regexes. Uma colisão futura só reaproveita a memorização quando o matching produzir esse mesmo conjunto de componentes de novo.

**Escape hatches:**

* `work plugin priority <nome> --before/--after <outro>` — antecipa uma preferência sem esperar a colisão ocorrer.
* `work plugin forget-choice --between <a>,<b>` — limpa uma memorização existente.

***

## 8. Atualização de plugins (ADR-0005)

* `work plugin outdated` — somente leitura, lista atual → disponível para todos os plugins instalados a partir de uma origem remota.
* `work plugin update <nome>` — individual.
* `work plugin update --all` — bulk, mostra diff de referência, confirma a menos que `--yes`.

**Mecânica de checagem.** Usa `git fetch` dentro do clone já existente do plugin (`~/.work/plugins/<nome>/source/`) — **nunca** `git pull` — para que a working tree que serve o entrypoint atualmente instalado jamais seja tocada por uma simples checagem. O manifesto no tip da referência remota rastreada é lido direto do object database (`git show <remote>/<branch>:plugin.json`), sem checkout. Só `update` (explícito, confirmado) move o checkout local para a nova referência resolvida.

Nenhuma tag git é exigida do autor — rastrear a branch e comparar apenas o campo `version` do manifesto é suficiente; o custo é um fetch incremental por plugin checado, pago só em `outdated`/`update` explícitos.

**Cache da auto-descrição.** O resultado do handshake `describe` (Seção 9) é cacheado associado ao `version` do plugin, para `source_type: git` — só é re-executado quando `update` aceita um `version` novo. Plugins `local-link` não têm evento de atualização nem disciplina confiável de bump de `version` durante desenvolvimento ativo — para esses, `describe` roda a cada invocação, sem cache (custa um spawn de processo local, não rede).

***

## 9. Contrato de execução de componentes (ADR-0000, ADR-0006)

Todo componente (starter, importer, linker, branch-strategy) segue o mesmo protocolo:

* **Entrada:** um único payload JSON via stdin, contendo o argumento bruto e o contexto relevante (ex: caminho do repositório já resolvido, quando aplicável).
* **Saída:** um único payload JSON via stdout, no formato acordado por tipo de componente (ex: um starter retorna `{ "repo_path": "...", "slug": "...", "base_branch": "..." }`, campos opcionais conforme as `capabilities` declaradas).
* **Código de saída:** `0` para sucesso; qualquer valor não-zero é tratado como "este componente não conseguiu resolver o argumento" (para starters) ou como falha de hook (para importers/linkers), sem derrubar o comando do Work — apenas reportando o erro ao usuário.
* **Sem estado compartilhado implícito:** cada invocação é um processo isolado; nada é assumido sobre o ambiente além de variáveis padrão do shell do usuário.
* **Ação `describe`:** no `install`/`update` (e a cada invocação para `local-link`, sem cache), o Work invoca o componente com uma ação `describe` sobre o mesmo contrato — a resposta do processo é a fonte de verdade sobre capabilities/versão, cacheada conforme Seção 8.

**Resolução de interpretador.** Campo opcional `runtime` no manifesto (`"python3"`, `"node"`, `"sh"`; ausente ou `"binary"` \= executável autocontido). O Work nunca depende de shebang ou do bit de execução do SO: quando `runtime` está declarado, invoca explicitamente `<runtime> <entrypoint> [args]`; quando ausente, executa o `entrypoint` diretamente. A presença do interpretador declarado é checada em preflight no `install`: se ausente do `PATH`, o Work falha com mensagem específica ("este plugin requer python3, não encontrado no PATH") em vez de deixar o subprocesso falhar de forma críptica.

`hooks.onFinalize` é uma lista de identificadores `<tipo>.<nome>` (ex: `importers.pr-metadata-importer`), resolvidos dentro dos componentes instalados do tipo correspondente.

***

## 10. TUI `work plugin` (ADR-0007)

`work plugin` sem subcomando abre uma TUI com: lista com nome, versão, origem, quantidade de componentes, estado habilitado/desabilitado. Atalhos:

* `[enter]` — detalhes do plugin selecionado.
* `[space]` — habilitar/desabilitar.
* `[u]` — atualizar.
* `[d]` — checar atualizações sob demanda (única ação que dispara rede; nunca automática ao abrir a tela).
* `[x]` — desinstalar.
* `[/]` — buscar.

É estritamente uma camada de apresentação sobre `work plugin install/list/enable/disable/update/uninstall <nome>` — nunca mantém estado próprio divergente do registro (Seção 5).

***

## 11. Integridade de origem sem assinatura formal (ADR-0008)

Como a origem de todo plugin instalado é sempre uma URL ou caminho local escolhido explicitamente pelo usuário (Seção 5), e a referência de conteúdo é fixada no momento da instalação, isso já dá integridade de conteúdo e reprodutibilidade suficientes para essa origem — sem exigir nenhuma etapa adicional de assinatura na v1. Não há hoje nenhum caminho no produto que instale um plugin sem uma origem fornecida diretamente pelo usuário.

***

## 12. Questões de design em aberto

1. **Contrato exato do componente `branch-strategy`.** O tipo de componente existe (Seção 4), mas o payload de entrada/saída exato (provavelmente recebe `repo_path` e retorna a chave da estratégia) e onde a convenção correspondente é configurada (dentro de `work.json`? um arquivo próprio?) ainda não estão definidos.
2. **Comportamento quando nenhum plugin fallback está habilitado.** Hoje, zero matches na camada `specific` cai no fallback único habilitado (Seção 7) — o caso em que nenhum fallback existe habilitado não tem um comportamento de erro definido: deve orientar o usuário a habilitar um, ou instalar o pacote de referência (Seção 6) novamente?
3. **Política de compatibilidade de versão do contrato JSON.** O manifesto já tem um campo `version` por pacote (Seção 4/ADR-0005), mas isso versiona o pacote, não o contrato entre Work e plugin em si. Falta definir a política de compatibilidade quando o Work evolui a versão desse contrato e plugins antigos ainda o implementam na versão anterior.