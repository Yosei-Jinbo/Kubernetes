# Copyright 2020 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

#  [Stage 1: builder]     Go ツールチェーン入りの image で scheduler をビルド
#          ↓ (binary だけコピー)                                                                                                           
#  [Stage 2: 本番 image]  distroless(最小 OS)に binary だけ載せる 

#GO_BASE_IMAGE … ビルド用の Go image。Makefile:34 で golang:$(GO_VERSION) が入る
ARG GO_BASE_IMAGE
#DISTROLESS_BASE_IMAGE … 実行用の超最小 image。デフォルトで gcr.io/distroless/static:nonroot(Google 製、shell も libc も入っていない) 
ARG DISTROLESS_BASE_IMAGE=gcr.io/distroless/static:nonroot
#ビルドを実行するマシンのアーキテクチャで走らせる指定。クロスコンパイル時のテクニックで、
#例えば amd64 ホストから arm64 向けに作るとき、コンパイラ自体は amd64(速い)で動かしたい 
FROM --platform=${BUILDPLATFORM} $GO_BASE_IMAGE AS builder

WORKDIR /workspace
COPY . .
#TARGETARCH … docker buildx が自動で入れる「作りたい image のアーキテクチャ」(amd64 / arm64 など) 
ARG TARGETARCH
# make build-scheduler … Makefile:57 を実行(main.goのコンパイル)
RUN make build-scheduler GO_BUILD_ENV='CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH}'


#Stage2: 本番のimage
#新しいステージ開始。先ほどの builder は破棄され、ここからは distroless image がベース(軽量イメージ)
FROM --platform=${BUILDPLATFORM} $DISTROLESS_BASE_IMAGE

#builder ステージから binary だけ取り出してコピー。ソースコードも Go ツールチェーンも最終 image には残らない。
WORKDIR /bin
COPY --from=builder /workspace/bin/kube-scheduler .

# UID:GID 65532(distroless の nonroot ユーザ)で実行
USER 65532:65532

#image を起動したら自動で /bin/kube-scheduler が走る
ENTRYPOINT ["/bin/kube-scheduler"]