"""Exercise the generated-script cache using the real importer's read-only gate."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

REPO = Path(__file__).resolve().parent.parent
DATABASE = REPO / "public/phaser/pokemon.db"
DIAGNOSTICS = "script_candidate_import_diagnostics.json"


def script_cache_paths(step_name: str) -> list[str]:
    workflow = (REPO / ".github/workflows/deploy.yml").read_text()
    step = workflow.split(f"      - name: {step_name}\n", 1)[1].split("\n      - name:", 1)[0]
    return sorted(line.strip() for line in step.splitlines()
                  if line.strip().startswith("server/scripted_events/"))


@unittest.skipUnless(DATABASE.is_file() and shutil.which("go"),
                     "Requires the canonical generated runtime assets and Go")
class RuntimeScriptCacheTest(unittest.TestCase):
    def test_restored_script_family_passes_check_and_missing_sidecar_fails(self) -> None:
        paths = script_cache_paths("Restore generated runtime assets")
        self.assertEqual(paths, script_cache_paths("Save generated runtime assets"))
        with tempfile.TemporaryDirectory(prefix="capturequest-script-cache-", dir="/var/tmp") as temporary:
            stage = Path(temporary)
            # Actions restores generated files over a checkout. Keep the tracked
            # manual overrides that the importer uses when compiling that family.
            tracked = subprocess.check_output(
                ["git", "ls-files", "-z", "server/scripted_events"], cwd=REPO, text=True)
            for name in filter(None, tracked.split("\0")):
                target = stage / name
                target.parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(REPO / name, target)
            for pattern in paths:
                sources = list(REPO.glob(pattern))
                self.assertTrue(sources, f"Missing generated cache input: {pattern}")
                for source in sources:
                    target = stage / source.relative_to(REPO)
                    target.parent.mkdir(parents=True, exist_ok=True)
                    if source.is_dir():
                        shutil.copytree(source, target, dirs_exist_ok=True)
                    else:
                        shutil.copy2(source, target)
            output = stage / "server/scripted_events"
            command = [
                "go", "run", "./cmd/import-script-candidates", "--check",
                "--sqlite", str(DATABASE), "--output", str(output / "scripts"),
                "--diagnostics", str(output / DIAGNOSTICS),
                "--release", os.environ.get("CAPTUREQUEST_POKEMON_RELEASE", "red"),
            ]
            restored = subprocess.run(command, cwd=REPO / "server", text=True,
                                      capture_output=True, timeout=180)
            self.assertEqual(restored.returncode, 0, restored.stdout + restored.stderr)

            # Reproduce the v3 cache omission and prove --check still rejects it,
            # without mutating the restored release to conceal the missing file.
            (output / DIAGNOSTICS).unlink()
            incomplete = subprocess.run(command, cwd=REPO / "server", text=True,
                                        capture_output=True, timeout=180)
            self.assertNotEqual(incomplete.returncode, 0)
            self.assertIn("would_write " + str(output / DIAGNOSTICS),
                          incomplete.stdout + incomplete.stderr)
            self.assertFalse((output / DIAGNOSTICS).exists())


if __name__ == "__main__":
    unittest.main()
