vllm serve Qwen/Qwen2.5-1.5B-Instruct \
  --gpu-memory-utilization 0.75 \
  --max-model-len 4096 \
  --max-num-seqs 4 \
  --max-num-batched-tokens 1024
