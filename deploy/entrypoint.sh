#!/usr/bin/env bash
set -euo pipefail

MODEL="${1:-Qwen/Qwen2.5-1.5B-Instruct}"

exec uv run -- vllm serve "$MODEL" \
  --gpu-memory-utilization 0.75 \
  --max-model-len 4096 \
  --max-num-seqs 4 \
  --max-num-batched-tokens 1024 \
  --enable-force-include-usage