# Docker Deployment Guide

This guide explains how to build and deploy the Rockbot Ad Server using Docker.

## Prerequisites

- Docker installed on your system
- Docker Compose (optional, for easier local deployment)

## Building the Docker Image

### Build locally:

```bash
docker build -t rockbot-adserver:latest .
```

### Build with specific tag:

```bash
docker build -t rockbot-adserver:v1.0.0 .
```

## Running the Container

### Basic run:

```bash
docker run -d \
  --name adserver \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -e DB_PATH=/app/data/adserver.db \
  rockbot-adserver:latest
```

### Using Docker Compose (Recommended):

```bash
docker-compose up -d
```

This will:
- Build the image
- Start the container
- Mount the `./data` directory for database persistence
- Expose port 8080

## Environment Variables

- `DB_PATH`: Path to the SQLite database file (default: `adserver.db`)
  - Example: `DB_PATH=/app/data/adserver.db`

## Volume Mounts

For production deployments, mount a volume to persist the database:

```bash
docker run -d \
  --name adserver \
  -p 8080:8080 \
  -v /path/to/persistent/data:/app/data \
  -e DB_PATH=/app/data/adserver.db \
  rockbot-adserver:latest
```

## Cloud Deployment

### AWS ECS / Fargate:

1. Build and push to ECR:
```bash
aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin <account-id>.dkr.ecr.us-east-1.amazonaws.com
docker build -t rockbot-adserver .
docker tag rockbot-adserver:latest <account-id>.dkr.ecr.us-east-1.amazonaws.com/rockbot-adserver:latest
docker push <account-id>.dkr.ecr.us-east-1.amazonaws.com/rockbot-adserver:latest
```

2. Use EFS or EBS for persistent storage for the database

### Google Cloud Run:

```bash
# Build and push to GCR
gcloud builds submit --tag gcr.io/PROJECT-ID/rockbot-adserver

# Deploy
gcloud run deploy rockbot-adserver \
  --image gcr.io/PROJECT-ID/rockbot-adserver \
  --platform managed \
  --region us-central1 \
  --allow-unauthenticated \
  --set-env-vars DB_PATH=/tmp/adserver.db \
  --memory 512Mi
```

Note: For Cloud Run, consider using Cloud SQL or a persistent volume for the database.

### Azure Container Instances:

```bash
# Build and push to ACR
az acr build --registry <registry-name> --image rockbot-adserver:latest .

# Deploy
az container create \
  --resource-group myResourceGroup \
  --name adserver \
  --image <registry-name>.azurecr.io/rockbot-adserver:latest \
  --dns-name-label adserver \
  --ports 8080 \
  --environment-variables DB_PATH=/app/data/adserver.db \
  --azure-file-volume-share-name data \
  --azure-file-volume-account-name <storage-account> \
  --azure-file-volume-account-key <key> \
  --azure-file-volume-mount-path /app/data
```

## Health Check

The container includes a health check that verifies the server is responding. Check health status:

```bash
docker ps  # Look for "healthy" status
```

## Logs

View container logs:

```bash
docker logs adserver
```

Follow logs in real-time:

```bash
docker logs -f adserver
```

## Stopping and Removing

```bash
# Stop
docker stop adserver

# Remove
docker rm adserver

# Or with docker-compose
docker-compose down
```

## Troubleshooting

### Database permissions issue:
Ensure the mounted volume has proper permissions:
```bash
chmod 755 ./data
```

### Port already in use:
Change the host port mapping:
```bash
docker run -p 8081:8080 ...
```

### Container exits immediately:
Check logs for errors:
```bash
docker logs adserver
```

## Production Considerations

1. **Database Persistence**: Use a managed database service (RDS, Cloud SQL, etc.) or persistent volumes
2. **Secrets Management**: Use cloud secrets manager for sensitive data
3. **Monitoring**: Add monitoring and logging (CloudWatch, Stackdriver, etc.)
4. **Scaling**: Consider using a managed database instead of SQLite for multi-instance deployments
5. **SSL/TLS**: Use a reverse proxy (nginx, traefik) or cloud load balancer with SSL termination
6. **Backup**: Implement regular database backups

