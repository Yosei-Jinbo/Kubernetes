kind create cluster --config kind-config.yaml #create a cluster with kind-config.yaml

#nginxのPodを作成
#nginxのイメージをpullする
docker pull nginx:latest
docker build -t my-nginx-app:latest ./my-nginx-app #build an image from Dockerfile

kind load docker-image my-nginx-app:latest
kubectl apply -f ./my-nginx-app/my-nginx-app.yaml