k8sタスク（3時間）
Docker・kind・kubectl・helm をインストール。kindで**3ノードクラスタ（control-plane 1 + worker 2）**を立てる。設定ファイル kind-config.yaml を書いて、単一コマンドで再現できるようにする。kubectl get nodes で3ノード見えるまで。nginx Podを1個立てて kubectl port-forward でブラウザから見えることを確認。
成果物 day01/

kind-config.yaml（3ノード構成）
setup.sh（クラスタ起動スクリプト、冪等に）
README.md：環境構成図（手書きでもOK、画像にして貼る）、実行コマンド、詰まった点のメモ