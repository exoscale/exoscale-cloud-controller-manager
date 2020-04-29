#!/bin/sh

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: exoscale-credential
  namespace: kube-system
type: Opaque
data:
  api-endpoint: '$(printf "%s" "$EXOSCALE_COMPUTE_ENDPOINT" | base64)'
  api-key: '$(printf "%s" "$EXOSCALE_API_KEY" | base64)'
  api-secret: '$(printf "%s" "$EXOSCALE_SECRET_KEY" | base64)'
EOF
