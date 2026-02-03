# Organização do Projeto e Pipelines

## Estrutura do Repositório
O projeto seguirá o padrão standard de Go (project-layout):
- `/cmd`: Entrypoints.
- `/internal`: Código privado da aplicação (maior parte da lógica).
- `/pkg`: Código reutilizável (se houver).

## Gestão de Dependências
- **Go Modules:** `go.mod` e `go.sum`.
- **Principais Libs:**
  - `github.com/aws/aws-sdk-go-v2`: Core SDK.
  - `github.com/aws/aws-sdk-go-v2/service/eks`: EKS.
  - `github.com/aws/aws-sdk-go-v2/service/sso`: Auth.
  - `github.com/charmbracelet/bubbletea`: UI.
  - `github.com/charmbracelet/lipgloss`: Estilos.
  - `gopkg.in/yaml.v3`: Parsing YAML.

## Build e Pipeline
- **Local:** `go build -o kboot.exe ./cmd/kboot`
- **CI/CD:** GitHub Actions (já existente, precisará de ajustes se o entrypoint mudar de `./main.go` para `./cmd/kboot/main.go`).

## Plano de Migração SDK
1. Instalar dependências do SDK v2.
2. Criar pacote `internal/aws` para isolar chamadas do SDK.
3. Substituir chamadas `exec.Command("aws", ...)` pelas funções do novo pacote.
