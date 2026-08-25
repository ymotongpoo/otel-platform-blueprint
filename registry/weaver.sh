#!/usr/bin/env bash
# Wrapper around the official weaver Docker image.
#
#   Usage: registry/weaver.sh {check|generate|diff <baseline-ref>|live-check}
set -euo pipefail

cd "$(dirname "$0")/.."
WEAVER_IMAGE="${WEAVER_IMAGE:-otel/weaver:v0.25.1}"

run_weaver() {
    docker run --rm -v "$(pwd)":/work -w /work "${WEAVER_IMAGE}" "$@"
}

case "${1:-}" in
    check)
        run_weaver registry check -r registry/model -p registry/policies
        ;;
    generate)
        run_weaver registry generate -r registry/model --templates registry/templates go sdk/semconv
        ;;
    diff)
        baseline="${2:?usage: weaver.sh diff <baseline-git-ref>}"
        run_weaver registry diff -r registry/model \
            --baseline-registry "https://github.com/ymotongpoo/otel-platform-blueprint.git@${baseline}[registry/model]"
        ;;
    live-check)
        docker run --rm -p 4317:4317 -v "$(pwd)":/work -w /work "${WEAVER_IMAGE}" \
            registry live-check -r registry/model
        ;;
    *)
        echo "usage: $0 {check|generate|diff <baseline-ref>|live-check}" >&2
        exit 1
        ;;
esac
