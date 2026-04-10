#ベースイメージの作成(python入りのLinuxイメージ)
FROM python:3.12-slim

#作業ディレクトリを指定
WORKDIR /app

#ローカルのapp.pyをコンテナ内の/appにコピー
COPY app.py .

#コンテナ起動時に実行するコマンド
CMD ["python", "app.py"]

#このコンテナが利用するポート
EXPOSE 8080