#!/bin/sh

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: exoscale-credentials
  namespace: kube-system
type: Opaque
data:
  api-key: '$(printf "%s" "$EXOSCALE_API_KEY" | base64)'
  api-secret: '$(printf "%s" "$EXOSCALE_API_SECRET" | base64)'
  api-zone: '$(printf "%s" "$EXOSCALE_ZONE" | base64)'
EOF
