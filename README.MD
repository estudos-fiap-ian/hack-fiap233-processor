# hack-fiap233-processor

Microsserviço em Go responsável pelo processamento assíncrono de vídeos. Consome jobs de uma fila SQS, extrai frames com FFmpeg, compacta em ZIP, armazena no S3 e notifica o usuário por e-mail.

## Fluxo

```
SNS → SQS → Processor → S3 Download → FFmpeg (1fps) → ZIP → S3 Upload → PostgreSQL → SMTP
```

## Arquitetura

Hexagonal (Ports & Adapters) com as seguintes camadas:

- **Domain** — modelos `VideoJob` e `ProcessingResult`
- **Application** — `VideoProcessorService` orquestra o pipeline
- **Adapters** — SQS, S3, FFmpeg, ZIP, PostgreSQL, SMTP, HTTP

## AWS

| Serviço | Uso |
|---------|-----|
| SQS | Fila de jobs (`hack-fiap233-video-processor`) |
| S3 | Input de vídeos e output de ZIPs (`hack-fiap233-videos-storage`) |
| ECR | Registro de imagens Docker |
| EKS | Cluster Kubernetes (`hack-fiap233-eks`) |
| Secrets Manager | Credenciais do banco de dados |

## Variáveis de ambiente

```env
# Obrigatórias
SQS_QUEUE_URL=https://sqs.us-east-1.amazonaws.com/.../hack-fiap233-video-processor
S3_BUCKET=hack-fiap233-videos-storage

# Banco de dados
DB_HOST=
DB_PORT=5432
DB_USERNAME=
DB_PASSWORD=
DB_NAME=

# E-mail (opcional)
SMTP_FROM=
SMTP_PASSWORD=
```

## Kubernetes

Deploy no EKS com liveness/readiness probe em `/health` (porta 8080). Métricas Prometheus expostas em `/metrics`.

```
Resource requests: 100m CPU / 128Mi RAM
Resource limits:   250m CPU / 256Mi RAM
```

## CI/CD

- **PR** — testes com cobertura mínima de 75% + build Docker
- **Deploy** — push para `main` constrói imagem, faz push para ECR e aplica no EKS via `kubectl rollout`

## Rodando localmente

```bash
go build -o processor .
./processor
```

> Requer FFmpeg instalado e as variáveis de ambiente configuradas.
