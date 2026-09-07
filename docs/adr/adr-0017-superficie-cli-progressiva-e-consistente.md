# ADR-0017: Superfície CLI Progressiva e Consistente

**Status:** Superseded by ADR-0019

**Substituída por:** ADR-0019 — Superfície CLI Progressiva com Home Estática e Ajuda Agrupada

**Data:** 2026-09-04

**Contexto de produto:** `docs/prd.md` — jornadas 7.1 a 7.7, RF-11 a RF-20, RF-25, RF-30, RF-31, RF-38, RF-45, RF-50 a RF-52 e RNF-10

**Governa:** `docs/add/add-0001-work-system-architecture.md`, Seções 5, 6, 7.2, 8 e 12

## Contexto

O Work delega escolhas difíceis à TUI para que iniciar, retomar e encerrar trabalho não exija memorizar uma árvore de comandos. A superfície proposta, porém, usava gramáticas diferentes em cada área administrativa: `policy` sem verbo, `locator list`, `roots list`, `plugin outdated`, `plugin update <nome>` e `convention set`. Reduzir toda a árvore à força produziria muitos comandos de topo ou flags que funcionariam como verbos, trocando profundidade sintática por ambiguidade.

É necessário separar a linguagem humana, descoberta por reconhecimento, da API direta usada por scripts, mantendo ambas sobre as mesmas operações.

## Decisão

Adotar **baixa profundidade de lembrança**, não profundidade sintática zero:

* `work` sem argumentos abre uma home TUI da qual todas as jornadas são alcançáveis.
* Os comandos cotidianos ficam no primeiro nível: `start`, `resume`, `archive`, `status`, `import` e `link`.
* `plugin`, `repository` e `convention` são hubs interativos. Sem subcomando, mostram a TUI do domínio e tornam visível o comando direto equivalente à operação concluída.
* Subcomandos administrativos formam a API de automação e seguem `work <caminho de recursos> <verbo>`: o verbo vem por último e cada termo anterior é um recurso no singular. Leitura usa `list` para coleções e `show` para um valor; `replace` substitui uma lista inteira.
* Não existem aliases oficiais para comandos ou recursos. Completion reduz digitação sem duplicar o vocabulário público.
* `work init` não faz parte da API pública. Bootstrap e configuração inicial acontecem sob demanda; configuração posterior é alcançável pelos hubs TUI.

### Superfície humana

```text
work
work start [SOURCE]
work resume [WORK]
work archive [WORK...]
work status [WORK]
work import [IMPORTER]
work link [LINKER] [VALUE]
work plugin
work repository
work convention
```

Sem alvo e com terminal interativo, `start`, `resume`, `archive`, `import` e `link` coletam ou oferecem as escolhas necessárias na TUI. Alvos explícitos pulam a seleção correspondente. `status` sem alvo usa o Work associado ao diretório corrente; fora de um Work exige alvo. Em stdin não interativo, qualquer valor obrigatório ausente falha com uso acionável, sem tentar abrir TUI.

### API direta

```text
work plugin list
work plugin install <SOURCE> [--link] [--as <ALIAS>]
work plugin enable <PLUGIN...>
work plugin disable <PLUGIN...>
work plugin update
work plugin update --check [PLUGIN...]
work plugin update <PLUGIN...>
work plugin update --all
work plugin uninstall <PLUGIN...>

work repository locator list
work repository policy list
work repository policy add <LOCATOR> [--before <LOCATOR> | --after <LOCATOR>]
work repository policy remove <LOCATOR...>
work repository policy move <LOCATOR> (--before <LOCATOR> | --after <LOCATOR>)
work repository policy replace <LOCATOR...>
work repository root list
work repository root add <PATH...>
work repository root remove <PATH...>
work repository root replace <PATH...>

work convention show
work convention set <CONVENTION>
```

`plugin update --check` é somente leitura; sem plugins explícitos, verifica todos os instalados. `plugin update` sem alvo abre a seleção interativa; com nomes explícitos atualiza aqueles plugins; `--all` atualiza todos os desatualizados após confirmação. `--check` e `--all` são mutuamente exclusivos.

`plugin install <SOURCE>` aceita origem remota ou caminho local. `--link` exige caminho local e escolhe o vínculo de desenvolvimento em vez de uma instalação fixada; a flag descreve o efeito, não apenas a origem.

### Semântica transversal

* Flags modificam operações; `add`, `remove`, `move`, `replace`, `install` e `uninstall` permanecem verbos.
* `--yes` confirma impactos já determinados e nunca escolhe alvo, modo ou valor em nome do usuário.
* Comandos de leitura (`status`, `list`, `show` e `update --check`) não alteram acesso recente, configuração, checkout ou proveniência.
* Todo comando de leitura aceita `--json`. Mutações têm saída e códigos de saída estáveis para automação.
* `remove` retira um item de uma coleção; `uninstall` remove um pacote; `archive` encerra um Work preservando seu estado; `replace` substitui uma coleção completa.

## Alternativas consideradas

* **Manter a superfície anterior.** Rejeitada porque cada ramo exigia aprender defaults, pluralização e verbos diferentes.
* **Achatar toda operação em comandos de topo.** Rejeitada por poluir o vocabulário cotidiano e apagar a relação entre recurso e operação.
* **Expressar mutações administrativas por flags.** Rejeitada porque flags como `--add-root` e `--remove-locator` funcionariam como verbos disfarçados, com composição e ajuda piores.
* **Oferecer aliases curtos como `repo`.** Rejeitada porque completion resolve o custo de digitação e um segundo nome aumenta a superfície reconhecível e documentável.
* **Manter `work view` apenas para links.** Rejeitada porque `view` não informa o que será exibido e limita a evolução natural de um resumo do Work. `work status` inclui estado de núcleo e links sem executar extensões.

## Consequências

**Positivas:** uma pessoa pode esquecer toda a árvore e se recuperar com `work`; comandos cotidianos permanecem rasos; a API não interativa ganha gramática regular e saídas estáveis; a TUI ensina a CLI por reconhecimento.

**Negativas / trade-offs:** operações administrativas podem chegar a três níveis depois de `work`; `work view`, `plugin outdated`, `repository roots` e `work init` deixam de existir antes da v1, exigindo que documentação e completion usem somente as formas canônicas.
