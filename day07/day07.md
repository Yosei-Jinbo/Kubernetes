## 雑多
### Kubernetesが持つリソース階層の整理
```
クラスタ (1 個)
├── Node (物理/仮想マシン、複数)
│   └── Pod が実行される場所
│
├── クラスタスコープリソース ← namespace に属さない
│   ├── Node
│   ├── PersistentVolume
│   ├── CustomResourceDefinition (CRD そのもの)
│   └── ...
│
└── Namespace (default, kube-system, ...)
    └── Namespace スコープリソース ← 特定 namespace に属する
        ├── Pod
        ├── Service
        ├── ConfigMap
        ├── NetworkTopology  ← ここ
        └── ...
```