kubectl proxy
curl -s http://localhost:8001/api/v1/nodes/topogang-cluster-control-plane/status | jq '.status.capacity'
curl -s -X PATCH \
  -H "Content-Type: application/json-patch+json" \
  http://localhost:8001/api/v1/nodes/topogang-cluster-control-plane/status \
  -d '[
    {"op": "add", "path": "/status/capacity/nvidia.com~1gpu", "value": "4"}
  ]' #/ は ~1 にエスケープ、~ は ~0 にエスケープ
kubectl describe node topogang-cluster-control-plane #反映の確認