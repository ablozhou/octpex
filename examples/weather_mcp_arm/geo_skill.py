"""
Geographic Entity Disambiguation & MCP Parameter Resolution Skill
Skill ID: geo_entity_disambiguation

Architecture:
Model-driven parameter resolution powered by the generic Octpex Skill Subsystem.
Prompts, schemas, and disambiguation guidelines are loaded dynamically from the
standard Claude SKILL.md definition (skills/geo_entity_disambiguation/SKILL.md),
allowing prompt tuning without altering application source code.
"""

import os
from typing import Dict, Any, Optional
from dataclasses import dataclass, asdict

# Import generic skill abstractions from Octpex Python SDK
from octpex.skill import parse_markdown_skill, Skill

SKILL_DIR_PATH = os.path.join(os.path.dirname(__file__), "skills", "geo_entity_disambiguation")

_DEFAULT_SKILL: Optional[Skill] = None


def get_geo_skill() -> Skill:
    """
    Returns the loaded Skill instance from skills/geo_entity_disambiguation/SKILL.md.
    """
    global _DEFAULT_SKILL
    if _DEFAULT_SKILL is None:
        if os.path.exists(SKILL_DIR_PATH):
            _DEFAULT_SKILL = parse_markdown_skill(SKILL_DIR_PATH)
        else:
            _DEFAULT_SKILL = Skill(
                id="geo_entity_disambiguation",
                name="Geographic Entity Disambiguation & Normalization",
                description="Resolves ambiguous natural language place names into canonical administrative hierarchies.",
                system_prompt="You are a Geographic Entity Disambiguation Skill for the Octpex Multi-Agent System.",
            )
    return _DEFAULT_SKILL


@dataclass
class NormalizedLocation:
    raw_input: str
    query_target: str
    canonical_name: str
    province_or_state: str
    country: str
    disambiguation_note: str
    clarification_needed: bool = False
    clarification_question: str = ""

    def to_dict(self) -> Dict[str, Any]:
        return asdict(self)


def normalize_location_entity(
    raw_input: str,
    slm_endpoint: Optional[str] = None,
    slm_model: Optional[str] = None,
) -> NormalizedLocation:
    """
    Resolves location parameters using generic model-driven skill execution.
    If no model endpoint is configured, returns pass-through without hardcoded rules.
    """
    cleaned = raw_input.strip()
    if not cleaned:
        return NormalizedLocation(
            raw_input=raw_input,
            query_target="",
            canonical_name="",
            province_or_state="",
            country="",
            disambiguation_note="No location provided.",
            clarification_needed=True,
            clarification_question="Please specify the target city or region to check weather for."
        )

    skill = get_geo_skill()
    profile = {}
    if slm_endpoint:
        profile["endpoint"] = slm_endpoint
    if slm_model:
        profile["model"] = slm_model

    result = skill.execute(cleaned, slm_profile=profile)
    return NormalizedLocation(
        raw_input=raw_input,
        query_target=result.get("query_target", cleaned),
        canonical_name=result.get("canonical_name", cleaned),
        province_or_state=result.get("province_or_state", ""),
        country=result.get("country", ""),
        disambiguation_note=result.get("disambiguation_note", "Direct pass-through without model-assisted disambiguation."),
        clarification_needed=result.get("clarification_needed", False),
        clarification_question=result.get("clarification_question", ""),
    )
