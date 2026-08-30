import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest


MODULE_PATH = Path(__file__).with_name("sync_extractor_assets.py")
SPEC = importlib.util.spec_from_file_location("sync_extractor_assets", MODULE_PATH)
sync = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(sync)


class AudioBundleValidationTest(unittest.TestCase):
    def test_requires_complete_hash_validated_bundle(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            artifacts = []
            for index in range(561):
                ogg = root / f"sound/pokemon/music/example-{index}.ogg"
                flac = root / f"sound/pokemon/music/example-{index}.flac"
                ogg.parent.mkdir(parents=True, exist_ok=True)
                ogg.write_bytes(f"ogg fixture {index}".encode())
                flac.write_bytes(f"flac fixture {index}".encode())
                artifacts.append({
                    "assetKey": f"music:{index}",
                    "distribution": {
                        "path": f"/sound/pokemon/music/example-{index}.ogg",
                        "sha256": hashlib.sha256(ogg.read_bytes()).hexdigest(),
                    },
                    "master": {
                        "path": f"/sound/pokemon/music/example-{index}.flac",
                        "sha256": hashlib.sha256(flac.read_bytes()).hexdigest(),
                    },
                })
            (root / "audio-render-manifest.json").write_text(
                json.dumps({
                    "schemaVersion": 2,
                    "renderProfile": {
                        "distribution": {
                            "sampleRate": 24000,
                            "channels": 1,
                            "codec": "ogg-vorbis",
                            "quality": 1,
                        }
                    },
                    "artifacts": artifacts,
                }),
                encoding="utf-8",
            )
            self.assertEqual(len(sync._validated_audio_artifacts(root)), 1122)

            destination = root / "public/sound/pokemon"
            self.assertEqual(sync._copy_audio_distributions(root, destination), 561)
            self.assertEqual(len(list(destination.rglob("*.ogg"))), 561)
            self.assertEqual(list(destination.rglob("*.flac")), [])
            self.assertTrue((destination / "audio-render-manifest.json").is_file())

            (root / "sound/pokemon/music/example-0.ogg").write_bytes(b"tampered")
            with self.assertRaisesRegex(RuntimeError, "hash mismatch"):
                sync._validated_audio_artifacts(root)

    def test_rejects_partial_or_traversing_bundle(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "audio-render-manifest.json").write_text(
                json.dumps({"artifacts": []}), encoding="utf-8"
            )
            with self.assertRaisesRegex(RuntimeError, "Expected 561"):
                sync._validated_audio_artifacts(root)


if __name__ == "__main__":
    unittest.main()
