import unittest
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from classify_deployment import classify_paths


class DeploymentClassifierTest(unittest.TestCase):
    def test_frontend_change_uses_frontend_lane(self) -> None:
        self.assertEqual(classify_paths(["src/pages/TitlePage.tsx"])["mode"], "frontend")

    def test_backend_change_uses_backend_lane(self) -> None:
        self.assertEqual(classify_paths(["server/internal/world/handler.go"])["mode"], "backend")

    def test_mixed_code_change_uses_code_lane(self) -> None:
        result = classify_paths(["src/App.tsx", "server/internal/server/server.go"])
        self.assertEqual(result["mode"], "code")
        self.assertEqual(result["full_data"], "false")

    def test_data_pipeline_change_forces_full_lane(self) -> None:
        result = classify_paths(["scripts/sync_extractor_assets.py"])
        self.assertEqual(result["mode"], "full")
        self.assertEqual(result["deploy_frontend"], "true")
        self.assertEqual(result["deploy_backend"], "true")

    def test_docs_only_change_does_not_deploy(self) -> None:
        self.assertEqual(classify_paths(["docs/DEPLOYMENT.md", "AGENTS.md"])["mode"], "none")

    def test_manual_mode_and_reset_override_auto_classification(self) -> None:
        self.assertEqual(classify_paths([], "frontend")["mode"], "frontend")
        self.assertEqual(classify_paths([], "backend")["mode"], "backend")
        self.assertEqual(classify_paths([], reset=True)["mode"], "full")

    def test_audio_manifest_is_generated_before_runtime_validation(self) -> None:
        bootstrap = (Path(__file__).resolve().parent / "bootstrap_capturequest.sh").read_text()
        generation = bootstrap.index('npm run audio:manifest')
        validation = bootstrap.index('scripts/validate_runtime_assets.py')
        self.assertLess(generation, validation)


if __name__ == "__main__":
    unittest.main()
