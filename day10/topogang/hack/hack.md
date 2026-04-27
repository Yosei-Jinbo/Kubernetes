# ここには開発環境のセットアップ、ローカルテスト、CI用のスクリプトや設定を書くらしい
- `Fake GPU capacity を patch。kind には GPU が無いので、capacity だけ「自称」させる。`というものがあるのでやってみる
- 方式1: 自分でproxyを立てて直接アクセスする
    1. 初めにkubernetesのリソースをいじるためにAPIサーバに接続する。`kubectl proxy`
    2. 現在のリソース状態(status)について確認 `curl <プロキシのip/url> | jq '.status.capacity`
    3. JSON PATCHによりそのURLの先を部分更新(これも`curl`コマンド)
    4. `kubectl describe node <control-node名>` によりCapacityに反映されたのかを確認する

- 方式2: Server Side Apply(SSA)方式でやってみる
    1. SSA方式ではそのオブジェクトに対する状態を.yamlファイルに宣言する(`fake-gpu.yaml`ファイルを参照して)
    2. この.yamlをクラスタに対して適用する
        ```bash
        kubectl apply --server-side \
            --subresource=status \
            --field-manager=fake-gpu-exxperiment \
            -f fake-gpu.yaml
        #--server-side: SSA方式の有効化、--subresource=status: statusサブリソースあてのApplyにする(URLをこの/statusとして送るようにする)
        #--field-manager=fake-gpu-experiment: 自分のマネージャー識別子を名乗る
        ```
    3. `kubectl describe node <control-node名> | grep -A8 Capacity` によりCapacityに反映されたのかを確認する
    4. `kubectl get node topogang-cluster-control-plane -o yaml --show-managed-fields | yq '.metadata.managedFields'`によりmanagedFields(自分が担当しているフィールド)を確認できる
        ```yaml
        -   manager: kubelet
            operation: Update
            subresource: status
            fieldsV1:
                f:status:
                f:capacity:
                    f:cpu: {}
                    f:memory: {}    #他のリソースはkubeletが管理していることが確認できる
                    ...

            - manager: fake-gpu-experiment
            operation: Apply              # ← Applyになっていると自分はSSAでこのフィールドを管理しているという意味
            subresource: status           #ちゃんとstatusサブリソースあてになっている
            fieldsV1:
                f:status:
                f:capacity:
                    f:nvidia.com/gpu: {}    #自分が所持しているmanagedFieldがnvidia.com/gpuだけになっている、他のフィールドは持っていないことが確認できる
        ```