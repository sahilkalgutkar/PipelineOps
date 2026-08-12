# PipelineOps on Kubernetes

Manifests here target either a local `kind` cluster (using the in-cluster
`postgres`/`redis` StatefulSet/Deployment in this directory) or EKS (where
you'd delete `20-postgres.yaml` / `21-redis.yaml` and point `DATABASE_URL`
at RDS and `REDIS_URL` at ElastiCache instead — see the root
[README](../../README.md#aws-deployment) for the full EKS walkthrough).

## Local: kind

```bash
# 1. Create the cluster
kind create cluster --name pipelineops

# 2. Install an ingress controller
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod --selector=app.kubernetes.io/component=controller \
  --timeout=120s

# 3. Build and load the app images into kind (kind can't pull from your local
#    Docker daemon directly)
docker build -t pipelineops-core-api:latest ./core-api
docker build -t pipelineops-ingestion:latest ./ingestion-service
docker build -t pipelineops-frontend:latest \
  --build-arg VITE_API_BASE_URL=http://pipelineops.local/api ./frontend
kind load docker-image pipelineops-core-api:latest --name pipelineops
kind load docker-image pipelineops-ingestion:latest --name pipelineops
kind load docker-image pipelineops-frontend:latest --name pipelineops

# 4. Create the namespace + secret (see 11-secret.example.yaml for the
#    literals it expects)
kubectl apply -f infra/k8s/00-namespace.yaml
kubectl create secret generic pipelineops-secrets -n pipelineops \
  --from-literal=DJANGO_SECRET_KEY="$(openssl rand -hex 32)" \
  --from-literal=POSTGRES_PASSWORD=pipelineops \
  --from-literal=DATABASE_URL="postgres://pipelineops:pipelineops@postgres:5432/pipelineops" \
  --from-literal=SLACK_WEBHOOK_URL="" \
  --from-literal=EMAIL_HOST_PASSWORD="" \
  --from-literal=TWILIO_AUTH_TOKEN=""

# 5. Apply everything else
kubectl apply -k infra/k8s/

# 6. Point pipelineops.local at the kind ingress and open it
echo "127.0.0.1 pipelineops.local" | sudo tee -a /etc/hosts
kubectl port-forward -n ingress-nginx svc/ingress-nginx-controller 8080:80 &
open http://pipelineops.local:8080
```

## Known simplifications (called out deliberately, not accidental)

- **Migrations run from every `core-api`/`celery-*` pod's entrypoint** on
  boot rather than a dedicated migration Job/initContainer. Fine for a
  single-region demo; a real rollout would gate migrations behind a
  `Job` that must complete before the Deployment rolls out.
- **`celery-beat` is a single replica by design** (`strategy: Recreate`) —
  running more than one instance double-fires every scheduled alert check.
- **No `DJANGO_CREATE_SUPERUSER` here** (unlike docker-compose) — create an
  admin user manually with `kubectl exec` once the pod is up:
  `kubectl exec -n pipelineops deploy/core-api -- python manage.py createsuperuser`.
- **Only `ingestion` has an HPA** — it's the one service on the request path
  that's expected to see bursty write throughput (many jobs pinging at once).
