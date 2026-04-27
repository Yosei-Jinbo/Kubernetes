#kubectl get nodes --show-labelsで確認
kubectl label node topogang-cluster-worker topology.example.io/rack=A topology.example.io/block=1
kubectl label node topogang-cluster-worker2 topology.example.io/rack=A topology.example.io/block=1
kubectl label node topogang-cluster-worker3 topology.example.io/rack=B topology.example.io/block=2