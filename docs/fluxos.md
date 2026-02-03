# Fluxos do Projeto - kboot

## Fluxo Principal de Inicialização (Boot)
```mermaid
flowchart TD
    Start([Início]) --> LoadConfig{Config Carregada?}
    LoadConfig -- Não --> ShowHelp
    LoadConfig -- Sim --> InitUI[Inicia TUI de Progresso]
    
    InitUI --> Workers[Inicia Worker Pool (5)]
    
    subgraph Parallel Execution
        Workers --> Job{Cluster Job}
        Job --> CheckSSO[Check AWS SSO Token]
        CheckSSO -- Expired --> Login[AWS SSO Login (Interactive)]
        CheckSSO -- Valid --> GetEKS[AWS SDK: DescribeCluster]
        
        Login -- Sucesso --> GetEKS
        Login -- Falha --> MarkError[Registra Erro]
        
        GetEKS -- Sucesso --> GenKube[Gera Kubeconfig File]
        GetEKS -- Falha --> MarkError
    end
    
    Workers --> WaitAll[Aguardar Todos]
    WaitAll --> Summary[Exibir Resumo de Erros]
    
    Summary --> CheckHeadless{Flag --no-k9s?}
    CheckHeadless -- Sim --> PrintPath[Imprime Caminho Config e Sai]
    CheckHeadless -- Não --> MergeConfig[Merge KUBECONFIG Env]
    MergeConfig --> LaunchApp[Executa k9s/App]
```
