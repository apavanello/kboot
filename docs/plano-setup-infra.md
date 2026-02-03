# Setup de Infraestrutura - kboot

## Objetivo
Ajustar o pipeline de build e release para suportar a nova estrutura e garantir entregas contínuas.

## Tarefas

- [ ] **Pipeline GitHub Actions**
    - [ ] Atualizar workflow de **Build/Test** para rodar em PRs
        - [ ] Ajustar path do main (`./cmd/kboot`)
        - [ ] Rodar `golangci-lint` no CI
    - [ ] Atualizar workflow de **Release** (Goreleaser?)
        - [ ] Configurar build matrix (Windows, Linux, macOS)
        - [ ] Gerar changelog automático

- [ ] **Testes de Integração (Local Infra)**
    - [ ] Criar script para mockar credenciais AWS localmente (para testes sem conta real)
    - [ ] Documentar como rodar testes que exigem credenciais reais (se houver)
