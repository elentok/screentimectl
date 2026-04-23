#!/usr/bin/env bash

set -euo pipefail

root=~/.local/share/piper-voices/
base_url="https://huggingface.co/rhasspy/piper-voices/resolve/v1.0.0"
voice_name="lessac"
voice_path="en/en_US/lessac/medium/en_US-lessac-medium.onnx"

uv tool install piper-tts

mkdir -p "$root"

if [ ! -e "$root/$voice_name.onnx" ]; then
  wget "$base_url/$voice_path?download=true" -O "$root/$voice_name.onnx"
fi

if [ ! -e "$root/$voice_name.onnx.json" ]; then
  wget "$base_url/$voice_path.json?download=true" -O "$root/$voice_name.onnx.json"
fi

echo "> piper --model "$root/$voice_name" --config "$root/$voice_name.json""
echo "Hello world" | piper --model "$root/$voice_name" --config "$root/$voice_name.json" --output_file output.wav
