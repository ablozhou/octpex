# Blog Publishing Multi-Agent Mesh Example

This example demonstrates a 5-arm collaborative workflow:

1. **Researcher Arm**: Queries background facts and technical pillars (`sub://{session}/research.json`).
2. **Planner Arm**: Organizes findings into a 5-chapter structured outline (`sub://{session}/outline.json`).
3. **Writer Arm**: Generates markdown drafts based on the outline and incorporates feedback (`sub://{session}/draft.md`).
4. **Critic Gate Arm**: Enforces objective Gate 2 verification (depth, checklist, keywords). In round 1, vetoes the draft to trigger an iterative revision loop; approves in round 2.
5. **Publisher Arm**: Injects YAML FrontMatter and outputs the finalized article to a shared directory or volume (`/data/blog/`).

## Running Locally

```bash
python examples/blog_publish_mesh/main.py --topic "Kubernetes Biomimetic Multi-Agent Fabric on Octpex" --output "./output/blog"
```

The resulting article will be written to `output/blog/blog_*.md`.

## Running in Kubernetes

Apply the Job manifest to your cluster:

```bash
kubectl apply -f examples/blog_publish_mesh/k8s-job.yaml
```

Follow the logs:

```bash
kubectl logs -n octpex-system job/octpex-blog-publishing-job -f
```

Clean up:

```bash
kubectl delete -f examples/blog_publish_mesh/k8s-job.yaml
```
