# Feature: Interface de Progresso (TUI) e Geração Kubeconfig

## Objetivo
Criar a interface visual de carregamento e o gerador de arquivos de configuração.

## Tarefas

- [x] **Gerador de Kubeconfig (internal/kube)**
    - [x] Migrar lógica de geração de YAML do `manage.go` para pacote dedicado
    - [x] Garantir que o output é válido e compatível com `kubectl`
    - [ ] Implementar flag `--headless` para apenas salvar arquivo e sair

- [ ] **Interface de Progresso (internal/ui)**
    - [ ] Criar modelo Bubbletea para tela de "Loading"
    - [ ] Implementar "Spinners" individuais para cada cluster/worker
    - [ ] Mostrar status final (Sucesso/Erro) para cada item
    - [ ] Integrar logs de erro na UI (ex: "Cluster X falhou: Network Timeout")

- [ ] **Integração Final**
    - [ ] Conectar Orquestrador com UI (enviar updates de progresso via canais/msgs)
    - [ ] Implementar lançamento do k9s (ou shell) após sucesso parcial
    - [ ] Validar fluxo completo: `kboot` -> UI Loading -> K9s
