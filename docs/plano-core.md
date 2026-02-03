# Implementação Core - kboot

## Objetivo
Implementar a espinha dorsal da aplicação: estrutura modular, orquestrador e integração com AWS SDK.

## Tarefas

- [x] **Refatoração Estrutural (Monolito -> Modular)**
    - [x] Mover lógica de `main.go` para `cmd/kboot/main.go`
    - [x] Criar pacote `internal/config` e migrar lógica de carga de YAML
    - [x] Criar abstrações/interfaces para Clients AWS (para facilitar testes)

- [x] **Módulo AWS (internal/aws)**
    - [x] Implementar `Client` struct que carrega config SDK
    - [x] Criar função `SSOLogin(profile)` usando SDK `service/sso`
    - [x] Criar função `DescribeCluster(name, region)` usando SDK `service/eks`
    - [x] Criar função `GetCallerIdentity()` usando SDK `service/sts`
    - [ ] Testar integração com `~/.aws/config` existente

- [ ] **Orquestrador de Boot (internal/app)**
    - [ ] Implementar lógica principal de boot
    - [ ] Criar **Worker Pool** (semáforo de 5 slots)
    - [ ] Implementar lógica de fallback e coleta de erros (não abortar no primeiro erro)
