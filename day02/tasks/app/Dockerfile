#GoをDockerfileとするにはStep1: ビルド, Step2: 実行という二つのフェーズに分けるらしい
#===== Step1 ビルド =====
FROM golang:latest AS builder
WORKDIR /app
#ビルドに利用するモジュール群をビルドするディレクトリ内に入れる
#go mod init myapp コマンドによりプロジェクト内に作成する
COPY go.mod ./ 
#RUN: イメージをビルドする最中にコンテナの中でコマンドを実行する(ビルド時に一回だけ)
#※CMD: ビルド後、コンテナを起動したときに実行される
#このRUNにより依存パッケージをダウンロードしておく
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o myapp main.go

#===== Step2 実行   =====
# ~5MB、シェルやデバッグツールも入っていて扱いやすい
#Go言語すべてのイメージはこのタイミングでは必要ない(Goの特徴)
FROM alpine:latest
WORKDIR /app
#ビルドしたファイルを実行できる箇所に持ってくる
COPY --from=builder /app/myapp ./
EXPOSE 8090
#実際にCMDによりコンテナ起動時に実行してアクセスできるようにする
CMD ["./myapp"]

#作成したイメージは docker imagesにより確認できる(大体は/var/lib/dockerに作られるらしい)