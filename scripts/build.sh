#!/usr/bin/env bash
set -euo pipefail

SERVICE_NAME=${SERVICE_NAME:-mini-web-app}
REGION=${REGION:-asia-northeast1}
PROJECT_ID=${PROJECT_ID:-mini-web-app-475006}
IMAGE_NAME="asia-northeast1-docker.pkg.dev/${PROJECT_ID}/${SERVICE_NAME}/main"
IMAGE_TAG="latest"
IMAGE="$IMAGE_NAME:$IMAGE_TAG"


# ====== Frontend ビルド実行 ======
frontend_dir="$(cd "$(dirname "$0")/../frontend" && pwd)"
if [ ! -d "$frontend_dir" ]; then
  echo "frontend ディレクトリが見つかりません。リポジトリの root で scripts/build.sh を実行してください。" >&2
  exit 1
fi
echo "Building frontend..."
cd "$frontend_dir"
rm -rf dist
mkdir -p dist
if [ -f package.json ]; then
  # TODO
  npm ci
  npm run build
else
  echo "package.json がありません。index.html のコピーのみします。"
  cp -f index.html dist/index.html
fi
cd -

# ====== Docker のビルド & Push ======
echo "Building Docker image: $IMAGE"
docker buildx build --platform linux/amd64 -t "$IMAGE" .

echo "Pushing Docker image to Artifact Registry..."
docker push "$IMAGE"

echo "アップロード完了"
