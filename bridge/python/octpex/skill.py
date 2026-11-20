"""
Generic Skill Management and Execution Subsystem
Provides model-driven skill registration and execution from Markdown definitions.
Arms can load skills declared in markdown files without writing custom Python handlers.
"""

import os
import json
import logging
import urllib.request
import urllib.error
from typing import Dict, Any, Optional, List, Union
from dataclasses import dataclass, field

logger = logging.getLogger("octpex.skill")


@dataclass
class Skill:
    """
    Represents a dynamic skill loaded from a Markdown definition.
    Supports multi-category DAG taxonomy and prerequisite dependency DAGs.
    """
    id: str
    name: str
    description: str
    system_prompt: str
    category: str = "general"
    categories: List[str] = field(default_factory=list)
    parent_category: Optional[str] = None
    dependencies: List[str] = field(default_factory=list)
    version: str = "1.0.0"
    file_path: Optional[str] = None
    parameters: Optional[Dict[str, Any]] = None
    metadata: Dict[str, Any] = field(default_factory=dict)

    def to_manifest_dict(self) -> Dict[str, Any]:
        """
        Serializes skill metadata and prompt into the standard ArmManifest representation for Octo-Brain.
        """
        parent_cat = self.parent_category
        if not parent_cat and "/" in self.category:
            parent_cat = self.category.split("/")[0]
        elif not parent_cat:
            parent_cat = self.category

        all_cats = list(self.categories)
        if self.category and self.category not in all_cats:
            all_cats.append(self.category)

        return {
            "id": self.id,
            "name": self.name,
            "category": self.category,
            "categories": all_cats,
            "parent_category": parent_cat,
            "dependencies": self.dependencies,
            "description": self.description,
            "system_prompt": self.system_prompt,
            "version": self.version,
            "metadata": self.metadata,
        }

    def to_manifest_dict(self) -> Dict[str, Any]:
        """
        Serializes skill metadata and prompt into the standard ArmManifest representation for Octo-Brain.
        """
        return {
            "id": self.id,
            "name": self.name,
            "description": self.description,
            "system_prompt": self.system_prompt,
            "version": self.version,
            "metadata": self.metadata,
        }

    def execute(
        self,
        input_payload: Union[str, Dict[str, Any]],
        slm_profile: Optional[Dict[str, Any]] = None,
        timeout: float = 4.0,
    ) -> Dict[str, Any]:
        """
        Executes the skill against the configured SLM profile using model-driven reasoning.
        """
        slm = slm_profile or {}
        endpoint = slm.get("endpoint") or os.environ.get("OCTPEX_SLM_ENDPOINT", "http://localhost:11434")
        model = slm.get("model") or os.environ.get("OCTPEX_SLM_MODEL", "qwen2.5:1.5b")

        raw_input_text = input_payload if isinstance(input_payload, str) else json.dumps(input_payload, ensure_ascii=False)

        # 1. Attempt model completion if endpoint is accessible
        if endpoint and model:
            res = self._call_model_endpoint(endpoint, model, raw_input_text, timeout)
            if res is not None:
                return res

        # 2. Resilient fallback when offline or model is not responding
        logger.debug(f"Skill '{self.id}' falling back to standard pass-through structure.")
        return self._build_pass_through_result(input_payload)

    def _call_model_endpoint(
        self,
        endpoint: str,
        model: str,
        user_input: str,
        timeout: float,
    ) -> Optional[Dict[str, Any]]:
        """
        Calls Ollama / OpenAI-compatible completion endpoints.
        """
        clean_endpoint = endpoint.rstrip("/")
        
        # Format 1: Ollama /api/generate
        ollama_url = f"{clean_endpoint}/api/generate"
        combined_prompt = f"{self.system_prompt}\n\nInput:\n{user_input}\n\nOutput JSON:"
        req_data = json.dumps({
            "model": model,
            "prompt": combined_prompt,
            "format": "json",
            "stream": False,
        }).encode("utf-8")

        req = urllib.request.Request(
            ollama_url,
            data=req_data,
            headers={"Content-Type": "application/json"},
            method="POST",
        )

        try:
            with urllib.request.urlopen(req, timeout=timeout) as resp:
                if resp.status == 200:
                    data = json.loads(resp.read().decode("utf-8"))
                    response_text = data.get("response", "").strip()
                    if response_text:
                        return json.loads(response_text)
        except Exception:
            pass

        # Format 2: OpenAI compatible /v1/chat/completions
        openai_url = f"{clean_endpoint}/v1/chat/completions"
        chat_payload = json.dumps({
            "model": model,
            "messages": [
                {"role": "system", "content": self.system_prompt},
                {"role": "user", "content": user_input},
            ],
            "response_format": {"type": "json_object"},
            "temperature": 0.1,
        }).encode("utf-8")

        req_chat = urllib.request.Request(
            openai_url,
            data=chat_payload,
            headers={"Content-Type": "application/json"},
            method="POST",
        )

        try:
            with urllib.request.urlopen(req_chat, timeout=timeout) as resp:
                if resp.status == 200:
                    data = json.loads(resp.read().decode("utf-8"))
                    content = data["choices"][0]["message"]["content"].strip()
                    if content:
                        return json.loads(content)
        except Exception:
            pass

        return None

    def _build_pass_through_result(self, input_payload: Union[str, Dict[str, Any]]) -> Dict[str, Any]:
        """
        Builds a generic clean pass-through dictionary if no model responds.
        """
        if isinstance(input_payload, dict):
            return dict(input_payload)
        cleaned = str(input_payload).strip()
        return {
            "query_target": cleaned,
            "canonical_name": cleaned,
            "province_or_state": "",
            "country": "",
            "disambiguation_note": "Pass-through resolution (local ganglion SLM offline).",
            "clarification_needed": False,
            "clarification_question": "",
        }


def parse_markdown_skill(file_path: str, default_id: Optional[str] = None) -> Skill:
    """
    Parses a Markdown skill definition file according to Claude / Anthropic Agent Skills specifications.
    Supports directory paths containing SKILL.md, as well as direct .md files.
    Supports YAML-style frontmatter between triple dashes (---) and markdown body as system prompt.
    """
    target_path = file_path
    if os.path.isdir(target_path):
        for candidate in ("SKILL.md", "skill.md", "README.md"):
            cand_path = os.path.join(target_path, candidate)
            if os.path.isfile(cand_path):
                target_path = cand_path
                break
        else:
            raise FileNotFoundError(f"No SKILL.md found in skill directory: {file_path}")

    if not os.path.exists(target_path):
        raise FileNotFoundError(f"Skill file not found: {target_path}")

    with open(target_path, "r", encoding="utf-8") as f:
        content = f.read()

    frontmatter_dict: Dict[str, Any] = {}
    body = content

    # Check for frontmatter delimited by ---
    if content.startswith("---"):
        parts = content.split("---", 2)
        if len(parts) >= 3:
            raw_fm = parts[1].strip()
            body = parts[2].strip()

            # Attempt parsing YAML if available, else simple key: value parser
            try:
                import yaml
                loaded = yaml.safe_load(raw_fm)
                if isinstance(loaded, dict):
                    frontmatter_dict = loaded
            except Exception:
                for line in raw_fm.splitlines():
                    if ":" in line:
                        k, v = line.split(":", 1)
                        frontmatter_dict[k.strip()] = v.strip().strip("\"'")

    fallback_id = default_id or (
        os.path.basename(os.path.dirname(os.path.abspath(target_path)))
        if os.path.basename(target_path).upper() in ("SKILL.MD", "README.MD")
        else os.path.splitext(os.path.basename(target_path))[0]
    )
    skill_id = str(frontmatter_dict.get("id") or frontmatter_dict.get("name") or fallback_id)
    name = str(frontmatter_dict.get("name") or skill_id.replace("_", " ").title())
    description = str(frontmatter_dict.get("description") or f"Skill loaded from {os.path.basename(target_path)}")
    version = str(frontmatter_dict.get("version") or "1.0.0")
    category = str(frontmatter_dict.get("category") or "general")
    parent_category = frontmatter_dict.get("parent_category")
    if not parent_category and "/" in category:
        parent_category = category.split("/")[0]

    raw_cats = frontmatter_dict.get("categories") or frontmatter_dict.get("tags") or []
    categories: List[str] = [str(c) for c in raw_cats] if isinstance(raw_cats, list) else []
    if category and category not in categories:
        categories.append(category)

    raw_deps = frontmatter_dict.get("dependencies") or frontmatter_dict.get("depends_on") or []
    dependencies: List[str] = [str(d) for d in raw_deps] if isinstance(raw_deps, list) else []

    return Skill(
        id=skill_id,
        name=name,
        description=description,
        system_prompt=body,
        category=category,
        categories=categories,
        parent_category=parent_category,
        dependencies=dependencies,
        version=version,
        file_path=os.path.abspath(target_path),
        metadata=frontmatter_dict,
    )


@dataclass
class SkillGroupMember:
    skill_id: str
    file_ref: Optional[str] = None
    role: str = ""
    depends_on: List[str] = field(default_factory=list)
    order: int = 0


@dataclass
class SkillGroup:
    """
    Represents a skill group / suite defined by a group metafile (e.g. group.yaml, bundle.json).
    Specifies member skills, DAG pipelines, and group-level metadata.
    """
    id: str
    name: str
    category: str
    description: str
    version: str = "1.0.0"
    members: List[SkillGroupMember] = field(default_factory=list)
    metafile_path: Optional[str] = None
    metadata: Dict[str, Any] = field(default_factory=dict)

    def to_manifest_dict(self) -> Dict[str, Any]:
        return {
            "id": self.id,
            "name": self.name,
            "category": self.category,
            "description": self.description,
            "version": self.version,
            "skills": [
                {
                    "skill_id": m.skill_id,
                    "file_ref": m.file_ref,
                    "role": m.role,
                    "depends_on": m.depends_on,
                    "order": m.order,
                }
                for m in self.members
            ],
            "metadata": self.metadata,
        }


def parse_skill_group_metafile(file_path: str) -> SkillGroup:
    """
    Loads a skill group definition metafile (group.yaml, group.json, or bundle.json).
    """
    if not os.path.exists(file_path):
        raise FileNotFoundError(f"Skill group metafile not found: {file_path}")

    with open(file_path, "r", encoding="utf-8") as f:
        content = f.read()

    data: Dict[str, Any] = {}
    if file_path.endswith((".yaml", ".yml")):
        try:
            import yaml
            data = yaml.safe_load(content) or {}
        except Exception:
            for line in content.splitlines():
                if ":" in line:
                    k, v = line.split(":", 1)
                    data[k.strip()] = v.strip().strip("\"'")
    else:
        data = json.loads(content)

    group_id = str(data.get("id") or data.get("name") or os.path.splitext(os.path.basename(file_path))[0])
    name = str(data.get("name") or group_id.replace("_", " ").title())
    category = str(data.get("category") or "general")
    description = str(data.get("description") or f"Skill group loaded from {os.path.basename(file_path)}")
    version = str(data.get("version") or "1.0.0")

    raw_skills = data.get("skills", [])
    members = []
    if isinstance(raw_skills, list):
        for idx, item in enumerate(raw_skills):
            if isinstance(item, str):
                members.append(SkillGroupMember(skill_id=item, order=idx + 1))
            elif isinstance(item, dict):
                s_id = str(item.get("id") or item.get("skill_id") or f"skill-{idx+1}")
                s_ref = item.get("ref") or item.get("file") or item.get("path")
                s_role = str(item.get("role") or "")
                s_deps = item.get("depends_on") or item.get("dependencies") or []
                s_order = int(item.get("order") or (idx + 1))
                members.append(
                    SkillGroupMember(
                        skill_id=s_id,
                        file_ref=s_ref,
                        role=s_role,
                        depends_on=[str(d) for d in s_deps] if isinstance(s_deps, list) else [],
                        order=s_order,
                    )
                )

    return SkillGroup(
        id=group_id,
        name=name,
        category=category,
        description=description,
        version=version,
        members=members,
        metafile_path=os.path.abspath(file_path),
        metadata=data,
    )


class SkillRegistry:
    """
    Manages and executes reusable skills loaded dynamically from Markdown definitions.
    """

    def __init__(self, base_dir: Optional[str] = None):
        self.base_dir = base_dir or os.getcwd()
        self._skills: Dict[str, Skill] = {}

    def register_skill(self, skill: Skill) -> None:
        """
        Registers an in-memory Skill instance.
        """
        self._skills[skill.id] = skill
        logger.info(f"Registered skill '{skill.id}' ({skill.name})")

    def load_skill_file(self, file_path: str, skill_id: Optional[str] = None) -> Skill:
        """
        Loads a single skill from a Markdown file path.
        """
        abs_path = file_path if os.path.isabs(file_path) else os.path.join(self.base_dir, file_path)
        skill = parse_markdown_skill(abs_path, default_id=skill_id)
        self.register_skill(skill)
        return skill

    def load_skills_from_directory(self, dir_path: str) -> List[Skill]:
        """
        Scans a directory for all .md files and registers them as skills.
        """
        abs_dir = dir_path if os.path.isabs(dir_path) else os.path.join(self.base_dir, dir_path)
        loaded = []
        if not os.path.isdir(abs_dir):
            return loaded

        for fname in sorted(os.listdir(abs_dir)):
            if fname.endswith(".md"):
                fpath = os.path.join(abs_dir, fname)
                try:
                    s = self.load_skill_file(fpath)
                    loaded.append(s)
                except Exception as e:
                    logger.warning(f"Failed to load skill from {fpath}: {e}")
        return loaded

    def load_from_manifest_declarations(self, declarations: List[Dict[str, Any]]) -> List[Skill]:
        """
        Loads skills declared in arm.yaml manifest (e.g. file, handler, id).
        """
        loaded = []
        for decl in declarations:
            s_id = decl.get("id")
            s_file = decl.get("file")
            if s_file:
                try:
                    s = self.load_skill_file(s_file, skill_id=s_id)
                    if "name" in decl:
                        s.name = decl["name"]
                    if "description" in decl:
                        s.description = decl["description"]
                    loaded.append(s)
                except Exception as e:
                    logger.warning(f"Error loading declared skill {s_id} from {s_file}: {e}")
        return loaded

    def get_skill(self, skill_id: str) -> Optional[Skill]:
        return self._skills.get(skill_id)

    def execute_skill(
        self,
        skill_id: str,
        input_payload: Union[str, Dict[str, Any]],
        slm_profile: Optional[Dict[str, Any]] = None,
    ) -> Dict[str, Any]:
        """
        Looks up a skill by ID and executes it against the provided SLM profile.
        """
        skill = self._skills.get(skill_id)
        if not skill:
            raise KeyError(f"Skill '{skill_id}' not found in registry. Registered skills: {list(self._skills.keys())}")
        return skill.execute(input_payload, slm_profile=slm_profile)

    def list_skills(self) -> List[Dict[str, Any]]:
        return [
            {
                "id": s.id,
                "name": s.name,
                "category": s.category,
                "parent_category": s.parent_category,
                "description": s.description,
                "version": s.version,
                "file_path": s.file_path,
            }
            for s in self._skills.values()
        ]

    def build_tree(self) -> Dict[str, Any]:
        """
        Organizes all registered skills into a hierarchical taxonomy tree.
        """
        tree: Dict[str, Any] = {
            "name": "Local Arm Skills",
            "path": "root",
            "is_leaf": False,
            "children": [],
        }

        for skill in self._skills.values():
            cat = skill.category or "general"
            segments = [seg for seg in cat.strip("/").split("/") if seg]
            curr = tree
            curr_path = ""

            for seg in segments:
                curr_path = f"{curr_path}/{seg}" if curr_path else seg
                found = None
                for ch in curr["children"]:
                    if ch["name"] == seg and not ch["is_leaf"]:
                        found = ch
                        break
                if not found:
                    found = {
                        "name": seg,
                        "path": curr_path,
                        "is_leaf": False,
                        "children": [],
                    }
                    curr["children"].append(found)
                curr = found

            curr["children"].append({
                "name": skill.name,
                "path": f"{curr_path}/{skill.id}",
                "is_leaf": True,
                "skill": skill.to_manifest_dict(),
            })

        return tree

    def list_manifest_dicts(self) -> List[Dict[str, Any]]:
        """
        Returns full skill definitions including system prompts to be published to Octo-Brain during registration.
        """
        return [s.to_manifest_dict() for s in self._skills.values()]

    def list_manifest_dicts(self) -> List[Dict[str, Any]]:
        """
        Returns full skill definitions including system prompts to be published to Octo-Brain during registration.
        """
        return [s.to_manifest_dict() for s in self._skills.values()]
