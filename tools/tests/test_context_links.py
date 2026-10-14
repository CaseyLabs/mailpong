"""Disposable CLI tests for the project context and Markdown link helpers."""
from __future__ import annotations

import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

TOOLS=Path(__file__).resolve().parents[1]
CONTEXT=TOOLS/"project-context.py"
CHECK=TOOLS/"check-doc-links.py"


class Checkout(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory(prefix="mailven docs ")
        self.root=Path(self.tmp.name)/"repo with spaces"
        self.root.mkdir()
        self.run_git("init","-q")
        self.run_git("config","user.email","test@example.invalid")
        self.run_git("config","user.name","Fixture")
        (self.root/"PLAN.md").write_text("# Project roadmap\n\n## Portfolio\nCandidates factual.\n\n## Progress\nNo automatic selection.\n\n## MAINT — Maintenance\nIntro facts.\nSibling project decision.\n\n### MAINT-5 — Helpers\n**Scope:** selected only.\n**Issues:** [F5b-3](ISSUES.md#f5b-3).\n**Checkpoint:** [custom](docs/plans/custom-checkpoint.md).\n\n### MAINT-50 — Similar ID\nMust not match.\n\n### MAINT-6 — Sibling milestone\nMust not appear.\n",encoding="utf-8")
        (self.root/"ISSUES.md").write_text("# Issues\n\n<a id=\"f5b-3\"></a>\n### F5b-3 — Rollback progress promise\nOnly this linked issue.\n\n<a id=\"other\"></a>\n### Other — Unlinked\nMust not appear.\n",encoding="utf-8")
        (self.root/"docs/plans").mkdir(parents=True)
        (self.root/"docs/plans/maint-5.md").write_text("# Default checkpoint\nMust not override an explicit link.\n",encoding="utf-8")
        (self.root/"docs/plans/custom-checkpoint.md").write_text("# Checkpoint\nCustom linked checkpoint.\n",encoding="utf-8")
        (self.root/"README.md").write_text("# Readme\n",encoding="utf-8")
        self.run_git("add","."); self.run_git("commit","-qm","fixture")

    def tearDown(self): self.tmp.cleanup()

    def run_git(self,*args):
        return subprocess.run(["git",*args],cwd=self.root,check=True,capture_output=True,text=True)

    def cli(self, script, *args, cwd=None):
        return subprocess.run([sys.executable,str(script),*args],cwd=cwd or self.root,capture_output=True,text=True)

    def test_context_scope_linked_files_and_path_spaces(self):
        r=self.cli(CONTEXT,"MAINT-5","--json")
        self.assertEqual(r.returncode,0,r.stderr)
        data=json.loads(r.stdout)
        rendered="\n".join(x for sec in data["sections"] for x in sec["lines"])
        self.assertIn("MAINT-5",rendered); self.assertIn("F5b-3",rendered); self.assertIn("Custom linked checkpoint",rendered)
        self.assertNotIn("Candidates factual",rendered)
        self.assertNotIn("Sibling milestone",rendered); self.assertNotIn("MAINT-50",rendered)
        self.assertNotIn("Must not appear",rendered); self.assertNotIn("Default checkpoint",rendered)
        self.assertIn("repo with spaces",data["repository"])

    def test_portfolio_view_and_line_limit(self):
        r=self.cli(CONTEXT,"--json","--max-lines","5")
        self.assertEqual(r.returncode,0,r.stderr)
        data=json.loads(r.stdout)
        self.assertTrue(data["truncated"])
        self.assertTrue(data["follow_up"])
        all_text="\n".join(line for s in data["sections"] for line in s["lines"])
        self.assertIn("Portfolio",all_text)
        self.assertNotIn("MAINT — Maintenance",all_text)

    def test_milestone_id_does_not_match_longer_hyphenated_id(self):
        plan=self.root/"PLAN.md"
        with plan.open("a",encoding="utf-8") as stream:
            stream.write("\n### MAINT-5-extra — Separate milestone\nUnselected detail.\n")
        result=self.cli(CONTEXT,"MAINT-5","--json")
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertNotIn("Unselected detail",result.stdout)

    def test_missing_plan_and_id_fail(self):
        self.assertNotEqual(self.cli(CONTEXT,"--plan","missing.md").returncode,0)
        self.assertNotEqual(self.cli(CONTEXT,"missing-id").returncode,0)

    def test_missing_referenced_issue_or_checkpoint_is_reported(self):
        plan=self.root/"PLAN.md"
        baseline=plan.read_text(encoding="utf-8")
        plan.write_text(baseline.replace("ISSUES.md#f5b-3","ISSUES.md#missing-issue"),encoding="utf-8")
        r=self.cli(CONTEXT,"MAINT-5")
        self.assertEqual(r.returncode,2)
        self.assertIn("issue anchor #missing-issue not found",r.stderr)
        plan.write_text(baseline.replace("docs/plans/custom-checkpoint.md","docs/plans/missing-checkpoint.md"),encoding="utf-8")
        r=self.cli(CONTEXT,"MAINT-5")
        self.assertEqual(r.returncode,2)
        self.assertIn("referenced checkpoint is missing",r.stderr)

    def test_context_works_inside_worktree_with_exact_milestone_id(self):
        wt=Path(self.tmp.name)/"worktree path with spaces"
        self.run_git("worktree","add","-qb","fixture-wt",str(wt))
        r=self.cli(CONTEXT,"MAINT-5","--json",cwd=wt)
        self.assertEqual(r.returncode,0,r.stderr)
        self.assertIn("MAINT-5",r.stdout)
        self.assertNotIn("MAINT-50",r.stdout)

    def test_markdown_links_paths_anchors_duplicates_and_site_routes(self):
        docs=self.root/"docs"; docs.mkdir(exist_ok=True)
        target=docs/"target page.md"
        target.write_text("<a id=\"old-name\"></a>\n# Repeat\n# Repeat\n",encoding="utf-8")
        (docs/"user-guide").mkdir()
        (docs/"user-guide/index.md").write_text("# Home\n",encoding="utf-8")
        (docs/"user-guide/route.md").write_text("# Café `code`\n\nSetext title\n-------------\n",encoding="utf-8")
        source=docs/"source.md"
        source.write_text("""[encoded](target%20page.md#repeat-1)
[raw spaces](target page.md#repeat)
[bad](missing.md)
[bad anchor](target%20page.md#absent)
[site](/docs/)
[site route](/docs/route#café-code)
[setext](/docs/route#setext-title)
[local](target%20page.md#old-name)
`[code](nope.md)`
```md
[fenced](nope.md)
```
[reference][ok]
[ok]: <target page.md#repeat>
""",encoding="utf-8")
        r=self.cli(CHECK,"docs/source.md","--json")
        self.assertEqual(r.returncode,1,r.stdout+r.stderr)
        self.assertEqual(json.loads(r.stdout)["error_count"],2,r.stdout)
        self.assertIn("missing link target",r.stdout)
        self.assertIn("missing anchor #absent",r.stdout)
        self.assertNotIn("nope.md",r.stdout)
        self.assertNotIn("repeat-1",r.stdout)

    def test_link_checker_missing_explicit_source_and_bounded_report(self):
        missing=self.cli(CHECK,"missing source.md")
        self.assertEqual(missing.returncode,1)
        self.assertIn("explicit Markdown source does not exist",missing.stdout)
        source=self.root/"lots.md"
        source.write_text("[a](missing-a.md)\n[b](missing-b.md)\n",encoding="utf-8")
        report=self.root/"full report.json"
        r=self.cli(CHECK,"--json","--max-errors","1","--json-output",str(report),"lots.md")
        self.assertEqual(r.returncode,1,r.stdout+r.stderr)
        brief=json.loads(r.stdout)
        full=json.loads(report.read_text(encoding="utf-8"))
        self.assertEqual(len(brief["errors"]),1)
        self.assertTrue(brief["errors_truncated"])
        self.assertEqual(full["error_count"],2)

    def test_explicit_sources_resolve_from_invocation_directory(self):
        docs=self.root/"docs"
        (docs/"nested.md").write_text("[root](../README.md#readme)\n[dir](plans/)\n",encoding="utf-8")
        result=self.cli(CHECK,"nested.md",cwd=docs)
        self.assertEqual(result.returncode,0,result.stdout+result.stderr)

    def test_site_directory_without_page_fails_but_repo_directory_is_valid(self):
        docs=self.root/"docs"
        (docs/"user-guide/empty-route").mkdir(parents=True)
        (docs/"route-links.md").write_text("[repo directory](plans/)\n[site directory](/docs/empty-route/)\n",encoding="utf-8")
        result=self.cli(CHECK,"docs/route-links.md","--json")
        self.assertEqual(result.returncode,1,result.stdout+result.stderr)
        report=json.loads(result.stdout)
        self.assertEqual(report["error_count"],1)
        self.assertEqual(report["errors"][0]["line"],2)

    def test_json_report_refuses_existing_file(self):
        output=Path(self.tmp.name)/"existing report.json"
        output.write_text("preserve me\n",encoding="utf-8")
        result=self.cli(CHECK,"README.md","--json-output",str(output))
        self.assertNotEqual(result.returncode,0)
        self.assertEqual(output.read_text(),"preserve me\n")

    def test_changed_includes_untracked_markdown_by_default(self):
        (self.root/"untracked file.md").write_text("[broken](absent.md)\n",encoding="utf-8")
        r=self.cli(CHECK)
        self.assertEqual(r.returncode,1,r.stdout+r.stderr)
        self.assertIn("untracked file.md:1",r.stdout)

    def test_changed_union_with_explicit_path(self):
        (self.root/"changed.md").write_text("[bad](missing.md)\n",encoding="utf-8")
        (self.root/"specified.md").write_text("[bad](missing-too.md)\n",encoding="utf-8")
        r=self.cli(CHECK,"--changed","specified.md")
        self.assertEqual(r.returncode,1,r.stdout+r.stderr)
        self.assertIn("changed.md:1",r.stdout)
        self.assertIn("specified.md:1",r.stdout)


if __name__=="__main__": unittest.main()
