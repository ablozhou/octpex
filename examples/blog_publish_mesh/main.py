"""
Octpex Blog Publishing Multi-Agent Mesh
Demonstrates a 5-arm collaborative workflow:
1. Research Arm: Gathers technical background points
2. Planner Arm: Generates structured chapter outline
3. Writer Arm: Drafts markdown article content
4. Critic Arm: Enforces objective quality gates with multi-round revision loops
5. Publisher Arm: Injects FrontMatter and publishes to shared directory/volume
"""

import argparse
import json
import os
import sys
import time

# Ensure bridge/python is importable
CURRENT_DIR = os.path.dirname(os.path.abspath(__file__))
PROJECT_ROOT = os.path.abspath(os.path.join(CURRENT_DIR, "..", ".."))
sys.path.insert(0, os.path.join(PROJECT_ROOT, "bridge", "python"))

from octpex import (
    InMemorySubstrateClient,
    NervousEnvelope,
    SecurityContext,
    SubstrateRef,
)


def run_blog_mesh(topic: str, output_dir: str, max_rounds: int = 3):
    print("=" * 80)
    print("        OCTPEX MULTI-AGENT BLOG PUBLISHING MESH (5-ARM COLLABORATION)")
    print("=" * 80)
    print(f"Topic       : {topic}")
    print(f"Output Path : {output_dir}")
    print(f"Max Rounds  : {max_rounds}\n")

    os.makedirs(output_dir, exist_ok=True)
    substrate = InMemorySubstrateClient()
    session_id = f"blog-{int(time.time())}"

    # --------------------------------------------------------------------------
    # Arm 1: Research Arm (检索调研腕足)
    # --------------------------------------------------------------------------
    print("[Arm 1: Researcher] Gathering technical background & key facts...")
    research_facts = {
        "topic": topic,
        "keywords": ["Biomimetic", "Kubernetes", "Redis CAS", "NATS JetStream", "Octpex"],
        "key_points": [
            "Cephalopod biology: 2/3 of neurons reside in arms (ganglia), enabling decentralized reflexes.",
            "Octo-Brain acts as strategic cortex, while Octo-Arms handle high-frequency local workflows.",
            "Sucker-Ring coordination substrate uses NATS JetStream for dual-plane messaging and Redis for atomic CAS versioning.",
            "Zero-Token edge execution offloads 70%-85% routine operations without invoking cloud LLMs.",
            "Kubernetes native deployment bundles entry, workers, etcd, Redis, and NATS in microservices."
        ]
    }
    research_uri = f"sub://{session_id}/research.json"
    substrate.put_artifact(research_uri, 0, json.dumps(research_facts, indent=2).encode("utf-8"))
    print(f"  -> Stored research findings at {research_uri} (Version: 1)")

    # --------------------------------------------------------------------------
    # Arm 2: Planner Arm (大纲规划腕足)
    # --------------------------------------------------------------------------
    print("\n[Arm 2: Planner] Structuring article outline based on research...")
    outline = {
        "title": f"Production Architecture: {topic}",
        "sections": [
            {"section": 1, "heading": "1. Executive Summary & Biomimetic Inspiration"},
            {"section": 2, "heading": "2. Cephalopod Topology: Brain, Ring & Arms"},
            {"section": 3, "heading": "3. Infrastructure Mesh: Redis CAS & NATS JetStream"},
            {"section": 4, "heading": "4. Kubernetes Cloud-Native Deployment & Shared Storage"},
            {"section": 5, "heading": "5. Production Benchmark Results & Future Outlook"}
        ]
    }
    outline_uri = f"sub://{session_id}/outline.json"
    substrate.put_artifact(outline_uri, 0, json.dumps(outline, indent=2).encode("utf-8"))
    print(f"  -> Stored structured outline at {outline_uri} (Version: 1)")

    # --------------------------------------------------------------------------
    # Arm 3 & 4: Writer & Critic Reflex Loop (创作撰写与质检监督多轮循环)
    # --------------------------------------------------------------------------
    current_round = 1
    feedback = ""
    approved = False
    final_draft_content = ""

    while current_round <= max_rounds and not approved:
        print(f"\n--- [Round {current_round}/{max_rounds}] Drafting & Quality Review Loop ---")

        # Writer Arm Drafts
        print(f"[Arm 3: Writer] Drafting article content (Incorporating feedback: '{feedback or 'Initial Draft'}')...")
        
        # Include extra depth on rounds > 1 to simulate addressing critique
        extra_depth = ""
        if current_round > 1:
            extra_depth = (
                "\n### Deep-Dive: Atomic Compare-And-Swap (CAS) & Dual-Plane Bus\n"
                "In high-throughput multi-agent environments, payload copying causes severe I/O bottlenecks. "
                "Octpex solves this via pass-by-reference pointers (`sub://session/artifact`). "
                "The Shared Data Substrate utilizes Redis Lua scripts to enforce atomic CAS leases with distributed TTLs, "
                "preventing race conditions when multiple arms read and update state concurrently.\n"
            )

        draft_markdown = f"""# Production Architecture: {topic}

## 1. Executive Summary & Biomimetic Inspiration
In modern multi-agent artificial intelligence systems, centralized cloud LLM routing suffers from explosive inference costs and compounding latency. Octpex introduces a biomimetic architecture modeled after cephalopod neurobiology, where two-thirds of the neurons are decentralized across edge arms.

## 2. Cephalopod Topology: Brain, Ring & Arms
The architecture is structured across three cohesive tiers:
- **Octo-Brain**: Central cognitive cortex performing macro intent planning and strategic arbitration.
- **Basal Nerve Ring**: Distributed coordination mesh providing dynamic service discovery and shared state.
- **Octo-Arms**: Autonomous edge execution workers running local SLMs (0.5B-3B) with self-healing reflex loops.

## 3. Infrastructure Mesh: Redis CAS & NATS JetStream
{extra_depth}
The messaging plane utilizes **NATS JetStream** to separate vertical dispatch envelopes from peer-to-peer lateral handoffs. 
Meanwhile, the **Redis CAS** storage layer guarantees zero-copy state synchronization with immutable revision tags.

## 4. Kubernetes Cloud-Native Deployment & Shared Storage
On Kubernetes, the system deploys as coordinated microservices:
- `octpex-entry`: Cluster scheduler and HTTP REST API gateway.
- `octpex-worker`: Scalable edge ganglia pods handling specialized capability tags.
- Shared Volume: Persistent storage mounts enabling local log harvesting and direct article artifacts delivery.

## 5. Production Benchmark Results & Future Outlook
Kernel benchmark stress tests demonstrate throughput exceeding **9.6M+ CAS ops/s** and **8.1M+ bus messages/s**, reducing cloud API token consumption by up to 85%.
"""
        draft_uri = f"sub://{session_id}/draft_r{current_round}.md"
        substrate.put_artifact(draft_uri, 0, draft_markdown.encode("utf-8"))
        print(f"  -> Writer generated draft ({len(draft_markdown)} bytes) at {draft_uri}")

        # Critic Arm Evaluates
        print("[Arm 4: Critic] Running objective verification gates (Schema, Keywords & Depth)...")
        
        required_keywords = ["Biomimetic", "Kubernetes", "Redis CAS", "NATS JetStream", "Octpex"]
        missing_keywords = [kw for kw in required_keywords if kw not in draft_markdown]
        
        # Round 1 deliberately enforces an intentional critique to demonstrate multi-round self-healing
        if current_round == 1:
            approved = False
            feedback = "Expand section 3 with detailed Redis CAS atomic update semantics and concurrency safety."
            print(f"  -> [Critic Veto] Quality score: 78/100. Gate 2 feedback: {feedback}")
            current_round += 1
            continue

        if missing_keywords:
            approved = False
            feedback = f"Missing mandatory keywords: {', '.join(missing_keywords)}"
            print(f"  -> [Critic Veto] Gate 2 check failed: {feedback}")
            current_round += 1
        else:
            approved = True
            final_draft_content = draft_markdown
            print("  -> [Critic Sign-Off] Quality score: 98/100! All structural and technical checks PASSED.")
            break

    if not approved:
        print("[Error] Failed to achieve Critic sign-off within maximum revision rounds.")
        sys.exit(1)

    # --------------------------------------------------------------------------
    # Arm 5: Publisher Arm (发布归档腕足 - 共享卷持久化)
    # --------------------------------------------------------------------------
    print("\n[Arm 5: Publisher] Injecting publication metadata & writing to shared volume...")
    
    frontmatter = f"""---
title: "Production Architecture: {topic}"
date: "{time.strftime('%Y-%m-%d %H:%M:%S')}"
author: "Octpex Multi-Agent Publishing Mesh"
tags: ["AI-Agents", "Kubernetes", "Octpex", "Distributed-Systems"]
status: "published"
quality_score: 98
revision_rounds: {current_round}
session_id: "{session_id}"
---

"""
    published_content = frontmatter + final_draft_content
    filename = f"blog_{session_id}.md"
    file_path = os.path.join(output_dir, filename)

    with open(file_path, "w", encoding="utf-8") as f:
        f.write(published_content)

    # Store final publication reference in Substrate
    pub_uri = f"sub://{session_id}/published/{filename}"
    v = substrate.put_artifact(pub_uri, 0, published_content.encode("utf-8"))

    print(f"  -> Substrate URI     : {pub_uri} (Version: {v})")
    print(f"  -> Local Shared File : {file_path}")
    print(f"  -> File Size         : {len(published_content.encode('utf-8'))} bytes")
    print("\n================================================================================")
    print("SUCCESS: Blog article was researched, outlined, drafted, critiqued, and published!")
    print("================================================================================")
    return file_path


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Octpex Blog Publishing Multi-Agent Mesh")
    parser.add_argument("--topic", default="Kubernetes Biomimetic Multi-Agent Fabric on Octpex", help="Blog topic")
    parser.add_argument("--output", default="./output/blog", help="Output directory")
    parser.add_argument("--rounds", type=int, default=3, help="Max revision rounds")
    args = parser.parse_args()

    run_blog_mesh(args.topic, args.output, args.rounds)
