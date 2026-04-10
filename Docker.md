# Dockerについて
## Dockerイメージを作る流れ
1. アプリ本体の用意
2. Dockerfileを書く(イメージの設計図)
    - 適当にPythonなんかでアプリを作る
3. docker build によりイメージをビルド
    - Dockerfile は「この順番でイメージを組み立ててね」という指示書です。
    - 例
    ```bash
    # ベースイメージ(Python 入りの軽量 Linux)
    FROM python:3.12-slim

    # 作業ディレクトリを設定
    WORKDIR /app

    # ローカルの app.py をコンテナ内にコピー
    COPY app.py .

    # コンテナ起動時に実行するコマンド
    CMD ["python", "app.py"]

    # このコンテナが使うポートを宣言
    EXPOSE 8080
    ```
4. イメージをビルド
    ```bash
    docker build -t my-app:v1 ./ #Dockerfileからイメージの作成

    docker images #作成したイメージの一覧

    docker run --rm -p 8080:8080 my-app:v1 #実際のアプリケーションの起動
    ```

## Dockerコンテナのライフサイクル
1. Docker Hubなどのリポジトリから公式のイメージを取得
    ```bash
    docker pull <イメージ名>
    ```
2. 取得イメージをもとにDockerfileを作成し、自分のイメージを作成
    ```bash
    (Dockerfileの作成)
    docker build -t <イメージ名> .
    ```

3. イメージからコンテナを作成
    ```bash
    docker create --name <コンテナ名> <イメージ名>
    ```

4. コンテナを起動
    ```bash
    docker start <コンテナ名>
    ```

5. 起動したコンテナを停止
    ```bash
    docker stop <コンテナ名>
    ```

6. コンテナをDockerイメージ化する(Commit)
    ```bash
    docker commit <コンテナ名> <イメージ名>:<タグ>
    ```

7. Docker Hubにイメージをアップロードする
    ```bash
    docker push <docker hub ID> / <イメージ名>:<タグ>
    ```