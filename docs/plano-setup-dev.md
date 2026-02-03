# Setup do Ambiente de Desenvolvimento - kboot

## Objetivo
Preparar o ambiente local para o desenvolvimento modular do kboot em Go, garantindo ferramentas de qualidade e padronização.

## Tarefas

- [x] **Configuração do Repositório**
    - [x] Criar estrutura de diretórios (`cmd/`, `internal/`, `pkg/`)
    - [x] Atualizar `.gitignore` para ignorar binários e configs locais
    - [ ] Criar `Makefile` ou `Taskfile` para automação (build, test, lint)

- [x] **Dependências (Go Modules)**
    - [x] Inicializar módulo (`go mod init`) se necessário (já existia)
    - [x] Adicionar **AWS SDK v2** (`aws-sdk-go-v2`, `service/eks`, `service/sso`, `config`)
    - [x] Adicionar **Bubbletea** e **Lipgloss** (`github.com/charmbracelet/...`)
    - [x] Executar `go mod tidy`

- [ ] **Code Quality & Linting**
    - [ ] Configurar **golangci-lint** (`.golangci.yml`)
    - [ ] Definir regras de formatação (`gofumpt` ou `goimports`)
    - [ ] Configurar Hooks de Git (pre-commit) para rodar lint/test antes do commit

- [ ] **VS Code / IDE**
    - [ ] Criar `.vscode/settings.json` para formatação automática
    - [ ] Criar `.vscode/launch.json` para debug do CLI com argumentos
