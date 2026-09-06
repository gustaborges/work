# ADR-0018: `work shell-init` e o protocolo `WORK_CD_FILE`

**Status:** Proposta

**Data:** 2026-09-06

**Contexto de produto:** `docs/prd.md` — RF-9; `specs/001-first-local-work/spec.md` FR-022, FR-023; roadmap §5.1

**Governa:** `docs/add/add-0001-work-system-architecture.md` §5 (superfície CLI) e §11 (contrato de processo); `specs/001-first-local-work/contracts/shell-integration.md`

**Relaciona-se com:** ADR-0017 (superfície CLI), ADR-0006 (execução direta de componentes), `research.md` R2

## Contexto

Ao final de `work start` a jornada esperada deixa o terminal **dentro do novo
worktree** (US1 cenário 1, FR-022). Um processo filho não pode alterar o diretório
de trabalho do shell pai; toda ferramenta que faz isso — `direnv`, `zoxide`,
`pyenv`, `fnm`, `jj` — instala um hook no shell do usuário.

O F1 precisa, portanto, de um novo comando público que emita esse hook. ADR-0017
fixou que **`work init` não é público** (bootstrap e configuração acontecem sob
demanda) e definiu a gramática administrativa `work <recurso> <verbo>`. Um emissor
de snippet de shell não é bootstrap nem se encaixa nessa gramática — ele não
administra recurso algum, apenas imprime texto. `research.md` R2 registrou a
decisão técnica (canal por arquivo temporário, wrapper opt-in) e marcou a
necessidade de um ADR curto para ratificar **nome, snippets por shell e o
protocolo** antes da v1. Este ADR faz essa ratificação.

## Decisão

### 1. Comando: `work shell-init <shell>`

* Nome **`shell-init`**, hífen, um único nível abaixo de `work`. É a única exceção
  reconhecida à gramática de ADR-0017: não tem verbo final porque não é uma
  operação sobre recurso, e sim um gerador de configuração para o shell.
* `<shell>` é obrigatório e assume os valores `bash`, `zsh`, `fish`, `powershell`.
  Valor ausente ou desconhecido → saída **2** (`usage`) listando os shells
  suportados.
* **Somente leitura.** Escreve o snippet em **stdout**, sai 0, não toca em arquivo
  nem estado. Aceita `--json`? Não — não há payload estruturado; é texto para
  `eval`.
* **Não aparece na home TUI** (`work` sem argumentos). É plumbing de instalação,
  não uma jornada.
* Uso pretendido, documentado no `--help` e no README:
  * `eval "$(work shell-init bash)"` / `zsh` no rc correspondente;
  * `work shell-init fish | source`;
  * `Invoke-Expression (& work shell-init powershell | Out-String)`.

### 2. Contrato do snippet (todos os shells)

O snippet define uma função/comando `work` que embrulha o binário real e:

1. cria um arquivo temporário privado `T`;
2. exporta, **apenas para o processo filho**, `WORK_CD_FILE=T` e
   `WORK_SHELL_INTEGRATION=1`;
3. executa o binário real com todos os argumentos originais, herdando
   stdin/stdout/stderr;
4. ao retornar: se `T` existe e é não vazio, faz `cd` para o caminho contido nele;
   em seguida remove `T`;
5. preserva o código de saída do filho como retorno da função.

O snippet **deve**: resolver o binário real sem recursão na função (`command`,
`builtin`, caminho absoluto ou `$WORK_REAL_BIN`); não vazar `WORK_CD_FILE` /
`WORK_SHELL_INTEGRATION` para o shell interativo além do filho; ser idempotente ao
ser carregado mais de uma vez.

### 3. Protocolo `WORK_CD_FILE` (lado do núcleo)

* Em `work start` **bem-sucedido** (saída 0), **após** o commit da linha em `works`,
  se `WORK_CD_FILE` estiver definido e não vazio, o núcleo escreve o **caminho
  absoluto do worktree** (`<dir>/worktree`) nesse arquivo. É a única coisa que o
  núcleo escreve ali.
* O núcleo **nunca** escreve em `WORK_CD_FILE` em falha, em cancelamento, ou para
  qualquer comando que não seja `start` (e, adiante, `resume`).
* Se `WORK_CD_FILE` está ausente/vazio numa criação bem-sucedida, o núcleo toma o
  **caminho FR-023**: sai 0, mantém o resumo de sucesso em stdout e imprime em
  **stderr** o aviso de que a sessão não foi movida, o caminho real do worktree em
  linha própria, e a linha `eval "$(work shell-init <shell detectado>)"`. O shell
  é inferido de `$SHELL` / presença de `$PSVersionTable`; se indeterminado, usa a
  forma `bash` e menciona os demais. O núcleo nunca afirma que um `cd` ocorreu.

### 4. Não objetivos (F1)

* Sem edição automática de arquivos rc (ADR-0017: sem `work init`; R2).
* Sem posicionamento para `cmd.exe`, `nushell`, `xonsh` — recebem o aviso FR-023.
* Sem spawn de subshell.

## Alternativas consideradas

* **Emitir `cd <path>` em stdout, usuário faz `eval "$(work start …)"`.** Rejeitada:
  destrói o stdout interativo normal (resumo de sucesso, TUI) e é frágil com a TUI.
* **Descritor de arquivo reservado (fd 3).** Rejeitada: configuração desajeitada em
  fish/PowerShell; o arquivo temporário é universal.
* **Spawn de um shell filho já dentro do worktree.** Rejeitada: aninha shells,
  quebra job control, perde histórico/sessão do pai.
* **`work init` que edita o rc.** Rejeitada por ADR-0017 e por ser intrusiva;
  `shell-init` apenas imprime e o usuário decide instalar.
* **Encaixar como `work shell init` (recurso `shell`, verbo `init`).** Rejeitada:
  `shell` não é um recurso administrável do Work e `init` reintroduziria o verbo
  que ADR-0017 removeu; `shell-init` como token único deixa claro que é um caso
  à parte.

## Consequências

**Positivas:** a jornada US1 termina dentro do worktree quando o hook está
instalado; a ausência do hook é reportada com honestidade e com instrução de
correção; o núcleo não precisa de fd reservado nem suja o stdout; o protocolo é o
mesmo em POSIX e PowerShell.

**Negativas / trade-offs:** `work shell-init` é um nome que foge à gramática de
ADR-0017 e precisa ser sempre documentado como exceção; o usuário tem um passo
manual de instalação; shells fora da matriz suportada nunca reposicionam.

## Acompanhamento

Ao promover para **Aceito**, referenciar este ADR em `add-0001` §5 e remover o
item "shell-init follow-up" do Constitution Check de
`specs/001-first-local-work/plan.md`.
