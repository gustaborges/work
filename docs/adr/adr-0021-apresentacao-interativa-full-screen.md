# ADR-0021: Apresentação Interativa Full-Screen para Jornadas Multi-Etapa

**Status:** Aceito

**Data:** 2026-09-07

**Contexto de produto:** `docs/prd.md` — RF-53, RF-55, RF-57, RF-64; `specs/003-terminal-ux-revamp/spec.md` FR-001, FR-007

**Relaciona-se com:** ADR-0009 (stack central), ADR-0020 (fronteira genérica de apresentação)

**Governa:** `docs/add/add-0001-work-system-architecture.md` §12.2/§12.3

**Realizada por:** `internal/present` (`Wizard`, `stepModel`), `internal/cli/start.go`

## Contexto

A ADR-0020 adotou, como primeira estrutura, uma **sessão inline delimitada por
etapa**: cada passo de `work start` / `work resume` / `work archive` era um
programa Bubble Tea independente renderizado no buffer corrente do terminal, que
colapsava para um recibo de uma linha ao ser aceito, e recusou explicitamente o
buffer alternativo.

Em uso real dois problemas se mostraram estruturais, não de ajuste:

1. **Fantasmas de renderização.** O renderer inline (padrão) do Bubble Tea perde a
   altura do frame em um resize de terminal e na transição confirmar→lista do
   `archive` (`Esc` para voltar): repinta o cabeçalho sem limpar as linhas
   antigas, empilhando o título da base ou a barra de abas `[Local] Remote` cinco
   ou seis vezes na tela. O hack `deSoftWrap` nos testes é um sintoma da mesma
   classe.
2. **Etapas independentes não compartilham tela.** Cada passo era seu próprio
   programa; os recibos das etapas já aceitas só reapareciam no buffer primário
   *depois* que aquele programa encerrava, e o programa seguinte entrava por cima.
   A jornada não tinha continuidade visual.

A ADR-0020 já previa que "um único programa que contenha toda a entrevista só
será considerado se uma necessidade futura de navegação retroativa justificar a
complexidade". A necessidade que apareceu não foi navegação retroativa e sim
**eliminar o fantasma** e **manter os recibos visíveis durante a jornada**.

## Decisão

Manter Bubble Tea e Lip Gloss e a fronteira genérica da ADR-0020, mudando apenas
a estrutura de renderização:

* Cada jornada roda como **um único programa full-screen** (buffer alternativo),
  um `present.Wizard` sobre etapas ordenadas. O buffer alternativo é ativado por
  `tea.View{AltScreen: true}` — não há opção de programa para isso no Bubble Tea
  v2.
* Um **clear + repaint completo a cada frame** torna o fantasma de resize e de
  volta-de-confirmação estruturalmente impossível.
* As quatro primitivas (`Input`, `Select`, `MultiSelect`, `Confirm`) deixam de
  ser `tea.Model` autônomas que chamam `tea.Quit` e passam a implementar um
  `stepModel` livre de domínio (`body`, `status`, `cursorPos`). O wizard compõe o
  quadro: uma régua `Primary` no topo, o título da jornada, o rastro de recibos
  aceitos e o corpo da etapa corrente. Nenhuma etapa encerra o programa; ela
  reporta um `status()` terminal e o wizard conduz a transição.
* As assinaturas públicas `Input` / `Select` / `MultiSelect` / `Confirm` são
  preservadas como wizards de uma etapa, então `resume` e `archive` migram sem
  mudança de código; apenas `start` é reestruturado para compor suas seis etapas
  condicionais em um wizard.
* **Ao encerrar**, o buffer primário é restaurado automaticamente e os recibos
  compactos das etapas aceitas são **reimpressos** no canal de UI (stderr, buffer
  primário), à frente das linhas estáveis de stdout que a CLI escreve em seguida.
  O "histórico do terminal" das métricas SC-001/SC-002 passa a significar essa
  reimpressão pós-encerramento.
* A fronteira de importação de `internal/present` é inalterada (stdlib + stack
  Charm + `colorprofile` + `x/term` + runewidth/uniseg + `internal/diag`); o
  teste `tests/contract/present_boundary_test.go` continua verde. Valores de
  opção de qualquer tipo `T` atravessam via genéricos e voltam à CLI por type
  assertion, sem `present` conhecer o tipo.

## Alternativas consideradas

* **Corrigir o renderer inline (rastrear altura, limpar antes de repintar).**
  Rejeitada: seria reimplementar parte do renderer do Bubble Tea e ainda
  frágil a cada nova transição de frame; o buffer alternativo com repaint total
  resolve a classe inteira.
* **Manter etapas inline independentes e só empilhar recibos manualmente.**
  Rejeitada: não resolve o fantasma no seletor ativo nem no `archive`, e a
  emenda de buffers entre programas continua imprevisível.
* **Um formulário único no buffer corrente (sem alt-screen).** Rejeitada pelo
  mesmo motivo da ADR-0020: o renderer inclusive nesse modo perde a altura do
  frame ao encolher (lista longa → recibo curto) e deixa lixo em scrollback.
* **Recusar o buffer alternativo (posição original da ADR-0020).** Revista: o
  histórico útil das escolhas aceitas é preservado pela reimpressão no buffer
  primário ao sair, então o motivo da recusa (apagar o histórico) não se aplica
  a esta estrutura.

## Consequências

**Positivas:** o fantasma de resize e de volta-de-confirmação é impossível por
construção; os recibos aceitos permanecem visíveis acima da etapa ativa durante
toda a jornada de `work start`; o seletor de base não mostra mais o SHA curto por
linha; a jornada é um app coerente; o `deSoftWrap` deixa de ser necessário para
novas telas.

**Negativas / trade-offs:** a coleta interativa agora ocupa a tela inteira
enquanto roda (o buffer primário volta intacto ao sair); o emulador VT dos testes
de integração precisou aprender o toggle de buffer alternativo (DECSET
1049/1047/47) e VPA; `work start` passou a resolver `--slug` / `--base` de flag
*depois* do passo de caminho quando o SOURCE é interativo, e `workspace.Persist`
move para depois do wizard (uma execução recusada não grava mais a raiz de
workspace).
