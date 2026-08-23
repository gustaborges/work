# ADR-0009: Stack Central — Go + Cobra + Bubble Tea + SQLite Embutido

**Status:** Aceito
**Data:** 2026-08-23
**Contexto de produto:** `docs-v2/prd.md` — RNF-5, RNF-6, Seção 10
**Supersede:** `docs/prd.md` (v1), Apêndice A.1

## Contexto

O rascunho v1 do PRD listava, em seu Apêndice A.1, a stack do núcleo (Go, Cobra, Bubble Tea, SQLite) como uma afirmação direta, sem alternativas nem justificativa registrada — uma decisão arquitetural apresentada como se fosse uma nota de rodapé do produto. O núcleo precisa: (a) ser distribuído como um binário único e portátil, sem exigir runtime externo instalado na máquina do usuário (RNF-6); (b) oferecer uma TUI navegável por teclado em toda superfície de seleção do produto (Seção 10 do PRD); (c) manter estado local auditável e consultável de forma ordenada, sem depender de varredura de filesystem (RNF-5, RF-11: listar trabalhos por `last_accessed_at`).

## Decisão

* **Go** como linguagem do núcleo — compila para um binário único, multiplataforma, sem exigir runtime externo instalado.
* **Cobra** como framework de CLI — padrão de mercado para CLIs em Go, usado por `kubectl`, `gh`, `hugo`, entre outros; reduz risco de manutenção por ser amplamente adotado e documentado.
* **Bubble Tea** como framework de TUI — é a opção idiomática em Go para as telas interativas de seleção que o produto exige em praticamente todo fluxo (plugins concorrentes, prefixos de branch, base branch, `work resume`, `work archive`, gestão de plugins).
* **SQLite embutido**, via driver Go puro sem dependência de CGO, para persistência de estado local — guardado dentro do layout descrito em `docs-v2/add/add-0001-work-system-architecture.md`, Seção 3.

## Alternativas consideradas

* **Node.js ou Python para o núcleo.** Rejeitada: ambos exigem um runtime externo instalado na máquina do usuário, contradizendo diretamente o objetivo de binário único e a RNF-6 de portabilidade — o próprio problema que este produto busca evitar para os plugins (ADR-0006) se tornaria um problema do próprio núcleo.
* **Rust.** Também produz binário único sem runtime externo, mas foi rejeitada por ter um ecossistema de CLI/TUI menos maduro e consolidado que o do Go para este caso de uso específico, sem um ganho claro que justifique o custo de curva de aprendizado e o ecossistema mais jovem de bibliotecas equivalentes a Cobra/Bubble Tea.
* **Arquivos JSON soltos em vez de SQLite embutido.** Rejeitada: listar trabalhos ordenados por último acesso (RF-11) sem variar de custo com o número de trabalhos exigiria, com arquivos soltos, varrer o filesystem e reordenar em memória a cada chamada — exatamente o problema que um banco embutido com índice resolve por construção.
* **CGO habilitado para o driver SQLite.** Rejeitada: exigiria um toolchain C disponível na máquina do usuário/no processo de build multiplataforma, reintroduzindo uma dependência externa que o driver puro Go evita.

## Consequências

**Positivas:** distribuição como binário único, sem dependência de runtime externo, em qualquer SO suportado; um único paradigma de framework (Cobra para comandos, Bubble Tea para seleção interativa) em todo o produto; consultas de estado ordenadas sem custo de varredura.

**Negativas / trade-offs:** acopla o núcleo a um único ecossistema de linguagem (Go) — aceitável porque essa escolha afeta apenas o núcleo, não os plugins, que permanecem livres de qualquer restrição de linguagem (ADR-0006); o driver SQLite embutido é mais uma dependência para manter atualizada, mitigado por ser puro Go e não exigir toolchain externo.
