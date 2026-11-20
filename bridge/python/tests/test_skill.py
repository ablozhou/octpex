import unittest
import tempfile
import os
import shutil

from octpex.skill import (
    Skill,
    SkillRegistry,
    parse_markdown_skill,
)


class TestSkillSubsystem(unittest.TestCase):
    def setUp(self):
        self.test_dir = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.test_dir)

    def test_markdown_skill_parsing(self):
        md_content = """---
id: test_geo_skill
name: Test Geographic Normalizer
category: geospatial/normalization
parent_category: geospatial
version: 1.2.0
description: Resolves location strings dynamically.
---

You are a Geographic Entity Resolution Skill.
Given an input location, return JSON with query_target.
"""
        file_path = os.path.join(self.test_dir, "test_skill.md")
        with open(file_path, "w", encoding="utf-8") as f:
            f.write(md_content)

        skill = parse_markdown_skill(file_path)
        self.assertEqual(skill.id, "test_geo_skill")
        self.assertEqual(skill.name, "Test Geographic Normalizer")
        self.assertEqual(skill.category, "geospatial/normalization")
        self.assertEqual(skill.parent_category, "geospatial")
        self.assertEqual(skill.version, "1.2.0")
        self.assertIn("You are a Geographic Entity Resolution Skill", skill.system_prompt)

    def test_skill_registry_and_tree(self):
        reg = SkillRegistry(base_dir=self.test_dir)

        s1 = Skill(
            id="geo_disambig",
            name="Geo Disambiguation",
            description="Normalizes places",
            system_prompt="Prompt 1",
            category="geospatial/normalization",
            version="1.0.0",
        )
        s2 = Skill(
            id="log_anomaly",
            name="Log Anomaly",
            description="Finds anomalies",
            system_prompt="Prompt 2",
            category="observability/logs",
            version="2.0.0",
        )

        reg.register_skill(s1)
        reg.register_skill(s2)

        self.assertEqual(len(reg.list_skills()), 2)
        tree = reg.build_tree()
        self.assertEqual(tree["name"], "Local Arm Skills")
        self.assertEqual(len(tree["children"]), 2)  # geospatial and observability

    def test_pass_through_execution_fallback(self):
        skill = Skill(
            id="test_fallback",
            name="Fallback Skill",
            description="Test fallback",
            system_prompt="System prompt",
        )
        # Execute without running SLM endpoint -> triggers resilient pass-through
        result = skill.execute("Washington State")
        self.assertEqual(result.get("query_target"), "Washington State")
        self.assertEqual(result.get("canonical_name"), "Washington State")


if __name__ == "__main__":
    unittest.main()
