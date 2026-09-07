# ADR-0020: Fronteira Genérica de Apresentação Interativa Inline

**Status:** Aceito — decisão sobre a estrutura de renderização (sessão inline por etapa e recusa do buffer alternativo) **substituída parcialmente pela ADR-0021**. A fronteira genérica CLI ↔ apresentação, os estados explícitos de etapa, a geometria estável, o ownership de streams e a fronteira de diagnóstico permanecem em vigor.

**Data:** 2026-09-07

**Contexto de produto:** `docs/prd.md` — RF-51 a RF-62, RF-64 e RNF-11; `specs/003-terminal-ux-revamp/spec.md`

**Relaciona-se com:** ADR-0009 (stack central), ADR-0019 (superfície CLI), ADR-0021 (apresentação full-screen)

**Governa:** `docs/add/add-0001-work-system-architecture.md` §12

**Realizada por:** `docs/add/add-0001-work-system-architecture.md` §12.2–12.5

## Contexto

O comportamento atual divide cada entrevista entre controles de terminal e validações de domínio executadas depois que o controle termina. Erros recuperáveis são então impressos como novas linhas, seletores encerram ainda expandidos e DTOs de Work atravessam a fronteira de apresentação. Esses sintomas não decorrem de uma limitação da stack aceita na ADR-0009; decorrem da ausência de uma fronteira que separe jornada, domínio, diagnóstico e estado visual.

Uma correção local de alturas e cores não impediria que os mesmos problemas reaparecessem em novos comandos. Também não é desejável mover sequenciamento e mutações para modelos de terminal, pois isso faria a camada de apresentação conhecer repositórios, branches, Works e plugins.

## Decisão

Manter Bubble Tea e Lip Gloss, conforme a direção da ADR-0009, atrás de uma camada de apresentação interativa própria do Work com estas responsabilidades:

* A camada CLI/aplicação controla a sequência da jornada, monta opções genéricas, fornece validações determinísticas e repetíveis, classifica erros, decide confirmações e executa mutações.
* A apresentação recebe somente títulos, descrições, opções genéricas, conteúdo de confirmação, formatação segura de recibos e callbacks de validação sem mutação. Ela não importa conceitos nem tipos de domínio do Work.
* Cada etapa interativa possui estados explícitos de edição/seleção, conclusão e cancelamento. Falha recuperável permanece no estado ativo e substitui o erro anterior; conclusão e cancelamento definem uma visão final compacta antes do encerramento normal.
* A primeira estrutura adotada é uma sessão inline delimitada por etapa. A CLI continua compondo as etapas em ordem; um único programa que contenha toda a entrevista só será considerado se uma necessidade futura de navegação retroativa justificar a complexidade.
* Seletores mantêm geometria estável durante o estado ativo e derivam seu limite das dimensões do terminal. A visão concluída pode e deve ser menor que a visão ativa.
* O foco usa estilo e uma coluna textual reservada de largura constante. Caixas de seleção, texto primário e metadados mantêm colunas e alturas previsíveis; medições consideram a largura exibida, não o tamanho em bytes.
* Input e writer de UI são fornecidos explicitamente à apresentação. UI interativa e diagnósticos humanos usam stderr/UI; stdout permanece reservado aos resultados estáveis dos comandos.
* A fronteira de diagnóstico transforma causas em mensagens públicas e acionáveis uma única vez. A cadeia de causas permanece inspecionável por mecanismo diagnóstico explícito, não pela saída humana normal.
* Huh pode permanecer como detalhe interno para campos simples durante a migração, mas não define o contrato da apresentação. Sua manutenção ou remoção futura não altera a CLI/aplicação nem os requisitos de produto.

## Alternativas consideradas

* **Corrigir cada prompt e seletor isoladamente.** Rejeitada porque mantém dependências de domínio, ownership de streams implícito e ciclos de erro inconsistentes.
* **Concentrar toda a entrevista em um único formulário.** Adiada porque simplificaria navegação retroativa e orçamento global de tela, mas deslocaria o sequenciamento para apresentação ou exigiria um protocolo assíncrono sem requisito atual.
* **Substituir toda a stack de terminal.** Rejeitada porque a stack atual já suporta operação inline, estado final, resize e estilos; a troca não resolve a fronteira arquitetural.
* **Entrar no buffer alternativo/full-screen.** Rejeitada porque apaga o histórico útil de escolhas aceitas e conflita com a continuidade esperada de um CLI diário.

## Consequências

**Positivas:** validações de domínio permanecem fora da renderização sem vazar erros para o histórico; controles são reutilizáveis e testáveis com dados genéricos; stdout e stderr ficam determinísticos; dependências visuais podem evoluir atrás da fronteira; novos seletores herdam tema, geometria e lifecycle coerentes.

**Negativas / trade-offs:** o Work passa a possuir uma camada adicional de adaptação e modelos visuais; validações fornecidas pela CLI precisam ser idempotentes e seguras para repetição; comportamento de terminal exige testes de viewport, Unicode, cor e PTY nas plataformas suportadas.
