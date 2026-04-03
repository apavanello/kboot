# Requisitos Não-Funcionais - kboot

## 1. Concorrência e Performance
- **Worker Pool:** A execução paralela de autenticação e validação deve ser limitada a **5 workers simultâneos**. Isso evita throttling na API da AWS e sobrecarga local.
- **Cache-First:** O sistema deve priorizar estritamente o uso de credenciais em cache fornecidas pelo AWS SSO/SDK. Chamadas de rede para login só devem ocorrer se o cache for inválido ou inexistente.

## 2. Resiliência e Tratamendo de Erros (Error Handling)
- **Isolamento de Falhas:** Um erro (timeout, falha de auth, rede) em uma thread/worker não deve impactar os outros workers.
- **Timeout:** Utilizar timeouts padrão do AWS SDK, permitindo retries automáticos do próprio SDK, desde que não bloqueie o progresso das outras goroutines.

## 3. Usabilidade e Feedback (CLI UX)
- **Relatório de Falhas:** Ao final da execução, o CLI deve apresentar explicitamente (stderr ou warning colorido) quais clusters/contas falharam e o motivo resumido, garantindo que o usuário saiba que falta algo.

## 4. Segurança
- **Persistência Segura:** Nenhuma credencial (Access Key/Secret) deve ser salva em texto puro pelo kboot. Toda a gestão de tokens é feita via AWS SDK Go v2 e o padrão `~/.aws/config`.

## 5. Compatibilidade
- **Go Version:** Manter compatibilidade com Go 1.21+.
- **OS:** Suporte contínuo a Windows, Linux e macOS.
