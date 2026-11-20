"""
Octpex Model Health & Diagnostic Probe Script
Validates connectivity, configuration, and inference latency for:
1. Local SLM (Ollama/vLLM) configured in configs/local.yaml
2. Cluster Frontier LLM (Gemini/OpenAI/Claude) configured in configs/cluster.yaml
"""

import os
import sys
import time
import json
import urllib.request
import urllib.error
import yaml


def load_yaml(filepath: str) -> dict:
    if not os.path.exists(filepath):
        return {}
    with open(filepath, "r", encoding="utf-8") as f:
        return yaml.safe_load(f) or {}


def probe_local_slm(config_path: str = "configs/local.yaml"):
    print("\n" + "=" * 80)
    print(" 1. DIAGNOSING LOCAL SLM (EDGE REFLEX GANGILA)")
    print("=" * 80)

    cfg = load_yaml(config_path)
    frontier = cfg.get("brain", {}).get("frontier", {})
    provider = frontier.get("provider", "ollama")
    base_url = frontier.get("base_url", "http://localhost:11434").rstrip("/")
    model = frontier.get("model", "qwen2.5-coder:7b")

    print(f"Config Source : {config_path}")
    print(f"Provider      : {provider}")
    print(f"Endpoint      : {base_url}")
    print(f"Target Model  : {model}")

    # Check connectivity
    tags_url = f"{base_url}/api/tags"
    print(f"\n[Step 1] Probing Ollama server status at {tags_url} ...")
    try:
        req = urllib.request.Request(tags_url, headers={"User-Agent": "OctpexProbe/1.0"})
        with urllib.request.urlopen(req, timeout=3.0) as resp:
            data = json.loads(resp.read().decode("utf-8"))
            installed_models = [m.get("name") for m in data.get("models", [])]
            print("Status        : ONLINE (HTTP 200)")
            print(f"Local Models  : {installed_models or 'None installed yet'}")

            # Check model presence
            model_matched = any(model in m or m in model for m in installed_models)
            if not model_matched:
                print(f"[WARNING] Model '{model}' was not found in installed models!")
                print(f"Suggestion    : Run `ollama pull {model}` to download it.")
            else:
                print(f"Model Check   : MATCHED ('{model}')")

    except urllib.error.URLError as e:
        print(f"Status        : FAILED TO CONNECT ({e.reason})")
        print("Remediation   :")
        print("  1. Ensure Ollama is running (`ollama serve`).")
        print(f"  2. Verify that {base_url} is reachable from this terminal.")
        return False
    except Exception as e:
        print(f"Status        : ERROR ({e})")
        return False

    # Perform a test inference
    gen_url = f"{base_url}/api/generate"
    print(f"\n[Step 2] Sending single inference generation test to {model} ...")
    start = time.time()
    payload = json.dumps({
        "model": model,
        "prompt": "Hello Octpex! Respond with only the word 'READY'.",
        "stream": False,
        "options": {"temperature": 0.1, "num_predict": 10}
    }).encode("utf-8")

    try:
        req = urllib.request.Request(
            gen_url,
            data=payload,
            headers={"Content-Type": "application/json"},
            method="POST"
        )
        with urllib.request.urlopen(req, timeout=15.0) as resp:
            res_json = json.loads(resp.read().decode("utf-8"))
            elapsed = time.time() - start
            output_text = res_json.get("response", "").strip()
            total_duration_ms = res_json.get("total_duration", 0) / 1e6

            print(f"Inference     : SUCCESS")
            print(f"Model Output  : \"{output_text}\"")
            print(f"Latency       : {elapsed:.2f}s (Engine duration: {total_duration_ms:.1f}ms)")
            return True
    except Exception as e:
        print(f"Inference     : FAILED ({e})")
        print(f"Suggestion    : Check if model '{model}' is fully pulled (`ollama pull {model}`).")
        return False


def probe_cluster_llm(config_path: str = "configs/cluster.yaml"):
    print("\n" + "=" * 80)
    print(" 2. DIAGNOSING CLUSTER FRONTIER LLM (CENTRAL CORTEX)")
    print("=" * 80)

    cfg = load_yaml(config_path)
    frontier = cfg.get("brain", {}).get("frontier", {})
    provider = frontier.get("provider", "gemini")
    base_url = frontier.get("base_url", "https://generativelanguage.googleapis.com/v1beta/openai").rstrip("/")
    api_key_env = frontier.get("api_key_env", "GEMINI_API_KEY")
    model = frontier.get("model", "gemini-1.5-flash")

    print(f"Config Source : {config_path}")
    print(f"Provider      : {provider}")
    print(f"Endpoint      : {base_url}")
    print(f"API Key Env   : {api_key_env}")
    print(f"Target Model  : {model}")

    api_key = os.getenv(api_key_env) or os.getenv("OPENAI_API_KEY")
    print(f"\n[Step 1] Checking API Key environment variable '${api_key_env}' ...")
    if not api_key:
        print(f"Status        : MISSING (${api_key_env} is not set in current environment)")
        print("Remediation   :")
        print(f"  Windows PowerShell: $env:{api_key_env} = \"your_api_key_here\"")
        print(f"  Linux / macOS     : export {api_key_env}=\"your_api_key_here\"")
        print("Note: In cluster environment, inject this key via Kubernetes Secret / ConfigMap.")
        return False
    else:
        masked_key = api_key[:4] + "..." + api_key[-4:] if len(api_key) > 8 else "***"
        print(f"Status        : DETECTED (Key: {masked_key})")

    # Perform Chat Completion request
    chat_url = f"{base_url}/chat/completions"
    print(f"\n[Step 2] Sending test ChatCompletion to {chat_url} ...")
    start = time.time()

    payload = json.dumps({
        "model": model,
        "messages": [
            {"role": "system", "content": "You are Octpex Central Cortex Diagnostic probe."},
            {"role": "user", "content": "Respond with only the word 'PONG'."}
        ],
        "temperature": 0.1,
        "max_tokens": 10
    }).encode("utf-8")

    req = urllib.request.Request(
        chat_url,
        data=payload,
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {api_key}"
        },
        method="POST"
    )

    try:
        with urllib.request.urlopen(req, timeout=20.0) as resp:
            data = json.loads(resp.read().decode("utf-8"))
            elapsed = time.time() - start
            choices = data.get("choices", [])
            content = choices[0]["message"]["content"].strip() if choices else "EMPTY"
            usage = data.get("usage", {})

            print("Inference     : SUCCESS")
            print(f"Model Output  : \"{content}\"")
            print(f"Token Usage   : Prompt={usage.get('prompt_tokens', 0)}, Completion={usage.get('completion_tokens', 0)}, Total={usage.get('total_tokens', 0)}")
            print(f"Roundtrip Time: {elapsed:.2f}s")
            return True
    except urllib.error.HTTPError as e:
        body = e.read().decode("utf-8", errors="ignore")
        print(f"Inference     : HTTP ERROR {e.code} ({e.reason})")
        print(f"Details       : {body}")
        if e.code == 404:
            print(f"Tip           : Model '{model}' may not be recognized by endpoint '{base_url}'. Check model naming.")
        elif e.code == 401 or e.code == 403:
            print("Tip           : Invalid API Key or quota exhausted.")
        return False
    except Exception as e:
        print(f"Inference     : CONNECTION FAILED ({e})")
        return False


if __name__ == "__main__":
    print("=" * 80)
    print("          OCTPEX LLM & SLM HEALTH & CONNECTIVITY PROBE")
    print("=" * 80)

    slm_ok = probe_local_slm("configs/local.yaml")
    llm_ok = probe_cluster_llm("configs/cluster.yaml")

    print("\n" + "=" * 80)
    print("                     DIAGNOSTIC SUMMARY")
    print("=" * 80)
    print(f"Local SLM Status   : {'[ONLINE]' if slm_ok else '[OFFLINE / ACTION REQUIRED]'}")
    print(f"Cluster LLM Status : {'[ONLINE]' if llm_ok else '[OFFLINE / ACTION REQUIRED]'}")
    print("=" * 80)
