# Kubernetesに自作スケジューラプラグインを作成する
## 単一スケジューラ方式でGoスケジューラプラグインを実装する
1. scheduler-pluginsrepositoryをクローン
    `git clone https://github.com/kubernetes-sigs/scheduler-plugins.git`
2. 自分のプラグイン用ファイル`<作業ディレクトリ>/scheduler-plugins/pkg/myscheduler/myscheduler.go`を作成
3. `myscheduler.go`の単体テスト + docker-imageに焼き付け
    ```bash
    go mod tidy
    go vet ./pkg/myscheduler/... ./cmd/scheduler/... #...はワイルドカードみたいなもの
    go build ./cmd/scheduler

    make local-image #ローカルイメージをホスト上にに作成
    ```

4. kindクラスタを作成して、イメージをクラスタにロード
    ```bash
    kind create cluster
    kind load docker-image <イメージ名>:<タグ> #ホスト上にあるdocker-imageをkindクラスタのノード内にコピーする

    docker images | grep scheduler-plugins #docker-imageのイメージ名:タグ名はこのように取得する
    ```

5. *kindクラスタのkind-control-planeコンテナ内で*`sched-cc.yaml,kube-scheduler.yaml`を設定変更
    1. コンテナ内に入るためにはVSCodeでコマンドパレットを開いて`Dev Containers: Attach to Running Container`
    1. もしくは`sudo docker exec -it $(sudo docker ps | grep control-plane | awk '{print $1}') bash`
    2. 元のファイルを退避
        `cp /etc/kubernetes/manifests/kube-scheduler.yaml /etc/kubernetes/kubescheduler.yaml.bak`
    3. `/etc/kubernetes/sched-cc.yaml`, `/etc/kubernetes/scheduler.conf`の編集 + `chmod 644 /etc/kubernetes/scheduler.conf`でscheduler.confが通常ユーザでも読めるようにする

6. kubeletが変更を監視して自動で変更を適用する
    - `kubectl get pods -n kube-system`により`kube-scheduler-kind-control-plane            1/1     Running`と見えるはず
    2. 

7. 実際にPodなどを作成してスケジューラの動作を確認
    - pod.yamlにより.yamlファイルの作成
    - kubectl apply -f pod.yamlによりpodの設定を読み込んでそのPodを作成する

## 複数スケジューラ方式でGoスケジューラプラグインを実装する。
5. 自作スケジューラを通常のホスト側で作成して、kubectl applyでKubernetes上に作成
    - マルチスケジューラ方式ではKubernetesのkind-control-planeとは別プロセスによりスケジューラが作成される。
    - 自作スケジューラ用のConfigMapを別namespaceに作る