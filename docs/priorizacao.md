# Priorização de Features - kboot

| Feature | Escopo | MosCoW | Esforço | Complexidade | Responsabilidade |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Login SSO Paralelo** | Funcional (Performance) | **Must Have** | Médio | Média | Backend (Go Concurrency) |
| **Tolerância a Falhas** | Não-Funcional (Resiliência) | **Must Have** | Médio | Média | Backend (Error Handling) |
| **Modo "Apenas Configuração"** | Funcional | **Should Have** | Simples | Baixa | CLI Args / Flow Control |
| **Feedback Visual de Progresso** | Funcional (UX) | **Should Have** | Simples | Baixa | CLI Output |
| **Worker Pool (5 threads)** | Não-Funcional (Perf) | **Must Have** | Médio | Média | Backend (Go Semaphores) |
| **Relatório Final de Erros** | Não-Funcional (UX) | **Must Have** | Simples | Baixa | CLI/Logging |
| **Cache Strategy** | Não-Funcional (Perf) | **Must Have** | Simples | Baixa | AWS SDK Config |

## Definições
- **Must Have:** Essencial para resolver a dor principal do usuário (lentidão e bloqueio total por falha única).
- **Should Have:** Importante para flexibilidade, mas o projeto funciona sem (com workarounds).
- **Esforço:** Baseado na quantidade de código e testes necessários.
- **Complexidade:** Baseado na dificuldade técnica (concorrência, tratamento de erros).
