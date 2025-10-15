#!/usr/bin/env bash
set -euo pipefail

SERVICE_NAME=${SERVICE_NAME:-mini-web-app}
REGION=${REGION:-asia-northeast1}
PROJECT_ID=${PROJECT_ID:-mini-web-app-475006}
IMAGE_NAME="asia-northeast1-docker.pkg.dev/${PROJECT_ID}/${SERVICE_NAME}/main"
IMAGE_TAG="latest"
IMAGE="$IMAGE_NAME:$IMAGE_TAG"

deploy_with_maintenance_flag() {
    local flag=$1
    echo ">>> Deploying with MAINTENANCE_MODE=${flag}"
    gcloud run deploy "${SERVICE_NAME}" \
        --image="${IMAGE}" \
        --region="${REGION}" \
        --update-env-vars="MAINTENANCE_MODE=${flag}" \
        --project="${PROJECT_ID}" \
        --quiet
}

stop_old_revisions() {
    echo ">>> Stopping old revisions..."
    # 現在のリビジョンを新しい順に取得
    revisions=()
    while IFS= read -r rev; do
        revisions+=("$rev")
    done < <(
        gcloud run revisions list \
            --service="${SERVICE_NAME}" \
            --region="${REGION}" \
            --project="${PROJECT_ID}" \
            --sort-by="~creationTimestamp" \
            --limit=2 \
            --format="value(metadata.name)"
    )

    if [ "${#revisions[@]}" -lt 2 ]; then
        echo "古いリビジョンが見つかりません。"
        return
    fi

    NEW_REVISION="${revisions[0]}"
    OLD_REVISION="${revisions[1]}"

    echo "最新: ${NEW_REVISION}"
    echo "古い: ${OLD_REVISION}"

    gcloud run services update-traffic "${SERVICE_NAME}" \
        --region="${REGION}" \
        --project="${PROJECT_ID}" \
        --to-revisions="${NEW_REVISION}=100,${OLD_REVISION}=0" \
        --quiet
}

set_latest_revision_100() {
    echo ">>> Setting latest revision to 100% traffic..."
    latest_rev=$(gcloud run revisions list \
        --service="${SERVICE_NAME}" \
        --region="${REGION}" \
        --project="${PROJECT_ID}" \
        --sort-by="~creationTimestamp" \
        --limit=2 \
        --format="value(metadata.name)" \
        | head -n 1)

    gcloud run services update-traffic "${SERVICE_NAME}" \
        --region="${REGION}" \
        --project="${PROJECT_ID}" \
        --to-revisions="${latest_rev}=100" \
        --quiet
}

# ===== 実行フロー =====
# 1. メンテナンス ON でデプロイ
deploy_with_maintenance_flag "on"

# 2. 古いリビジョンを停止
stop_old_revisions

echo "古いリビジョン停止を反映させるために 60 秒待機..."
sleep 60

# 3. メンテナンス OFF で再デプロイ
deploy_with_maintenance_flag "off"
set_latest_revision_100

echo "デプロイ完了"
