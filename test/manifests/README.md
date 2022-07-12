## Refresh tests resources

```bash
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update
helm template --namespace ingress-nginx ingress-nginx ingress-nginx/ingress-nginx > manifests/ingress-nginx.yml
```