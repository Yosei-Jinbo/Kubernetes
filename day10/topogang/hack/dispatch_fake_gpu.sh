#patch_fake_gpu.shで行ったgpu resourceの追加をなかったことにする
curl -s -X PATCH \
  -H "Content-Type: application/json-patch+json" \
  http://localhost:8001/api/v1/nodes/topogang-cluster-control-plane/status \
  -d '[
    {"op": "remove", "path": "/status/capacity/nvidia.com~1gpu"}
  ]'

curl -s -X PATCH \
  -H "Content-Type: application/json-patch+json" \
  http://localhost:8001/api/v1/nodes/topogang-cluster-control-plane/status \
  -d '[
    {"op": "remove", "path": "/status/capacity/nvidia.com~1gpu"},
    {"op": "remove", "path": "/status/allocatable/nvidia.com~1gpu"}
  ]'