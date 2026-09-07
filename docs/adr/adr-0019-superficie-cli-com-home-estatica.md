# ADR-0019: Superfície CLI Progressiva com Home Estática e Ajuda Agrupada

**Status:** Aceito

**Data:** 2026-09-07

**Substitui:** ADR-0017 — Superfície CLI Progressiva e Consistente

**Contexto de produto:** `docs/prd.md` — jornadas 7.1 a 7.7, RF-11 a RF-20, RF-25, RF-30, RF-31, RF-38, RF-45, RF-50 a RF-64 e RNF-10 a RNF-11

**Governa:** `docs/add/add-0001-work-system-architecture.md`, Seções 5, 6, 7.2, 8 e 12

**Realizada por:** `docs/add/add-0001-work-system-architecture.md` §12

## Contexto

A ADR-0017 estabeleceu corretamente uma superfície humana rasa, hubs interativos para administração e uma API direta regular. Sua home selecionável, porém, mistura descoberta com execução: retém um quadro interativo grande no histórico, duplica o inventário vivo de comandos e exige um fluxo de seleção até para quem precisa apenas recordar a sintaxe.

O objetivo de produto permanece o mesmo — recuperar-se sem memorizar a árvore de comandos — mas a home TUI deixou de ser o mecanismo adequado. O Work precisa apresentar identidade própria, encaminhar para uma ajuda completa e manter cada jornada diretamente invocável, sem desfazer a gramática e as garantias de automação já aceitas.

## Decisão

Manter a gramática, os comandos cotidianos, os hubs administrativos e a semântica transversal da ADR-0017, substituindo apenas a decisão de home:

* Em terminal interativo, `work` sem argumentos renderiza uma saída estática com o wordmark `WORK` em arte de terminal, tagline e orientação para `work --help`; não abre seletor e sai 0.
* Em terminais com cor verdadeira e contraste adequado, o wordmark usa um degradê do acento primário `#11A8CD` ao secundário `#8B7CF6`. Saídas estreitas, monocromáticas ou não interativas usam `WORK` em forma compacta e legível.
* `work --help` é a fonte de descoberta: inclui marca, uso, opções e exatamente os comandos registrados no binário, agrupados por contexto. Grupos vazios e comandos ainda não implementados não aparecem.
* A taxonomia mínima distingue comandos cotidianos/globais, comandos dependentes de um Work materializado, administração e setup/plumbing. A classificação não altera a gramática dos comandos.
* Em modo não interativo, `work` sem argumentos preserva a falha de uso e o código 2; `work --help` sai 0. Nenhum dos dois caminhos emite controles interativos quando as streams relevantes não são TTYs.
* Permanecem válidos os comandos cotidianos `start`, `resume`, `archive`, `status`, `import` e `link`; os hubs interativos `plugin`, `repository` e `convention`; a ausência de aliases; a pureza de leituras; `--json`; `--yes`; e os contratos estáveis de stdout, erro e código de saída.
* Toda jornada pública continua diretamente invocável por comando documentado. A home e a ajuda facilitam reconhecimento; não se tornam um caminho de execução alternativo.

### Superfície humana preservada

```text
work
work --help
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

Sem alvo e com terminal interativo, `start`, `resume`, `archive`, `import` e `link` coletam ou oferecem as escolhas necessárias. Alvos explícitos pulam somente a seleção correspondente. `status` sem alvo usa o Work associado ao diretório corrente. Em uso não interativo, valor obrigatório ausente falha com uso acionável, sem abrir controles interativos.

### API direta preservada

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

Os termos anteriores ao verbo continuam sendo recursos no singular; leituras usam `list` para coleções e `show` para um valor; `replace` substitui uma coleção completa. `plugin update --check` é somente leitura; `plugin update` sem alvo abre seleção; nomes explícitos ou `--all` determinam o conjunto a atualizar, com confirmação aplicável. `plugin install --link` exige caminho local. `work init` e aliases continuam fora da API pública.

## Alternativas consideradas

* **Manter a home TUI e corrigir apenas seu quadro final.** Rejeitada porque continuaria duplicando descoberta e execução, além de exigir interação para consultar o produto.
* **Fazer a home estática listar todos os comandos.** Rejeitada porque duplicaria o inventário que já pertence à ajuda e poderia divergir dele.
* **Imprimir apenas a ajuda completa em `work`.** Rejeitada porque uma entrada curta preserva identidade e orientação sem despejar informação quando o usuário apenas testa o comando.
* **Alterar junto a gramática administrativa.** Rejeitada porque o problema observado não invalida a baixa profundidade de lembrança nem os verbos e recursos já aceitos.

## Consequências

**Positivas:** descoberta e execução ficam separadas; a ajuda não anuncia comandos inexistentes; o comando vazio deixa histórico curto; a identidade visual aparece sem introduzir uma segunda árvore de navegação; scripts preservam o comportamento detectável do comando vazio.

**Negativas / trade-offs:** iniciar uma jornada a partir do comando vazio exige consultar a ajuda e executar o comando indicado; a renderização de marca precisa de formas responsivas e sem cor; documentação F1/F2 que descreve a antiga home permanece histórica e deve ser explicitamente supersedida pelo contrato desta feature.
