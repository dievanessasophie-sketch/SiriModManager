"""One-time publication of the unsigned binaries approved on 2026-10-03.

Runs only on the dedicated release branch. Never rebuilds or signs binaries,
never replaces an existing release and never moves an existing version tag.
"""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import zipfile

REPO = "dievanessasophie-sketch/SiriModManager"
TAG = "v0.8.0"
SOURCE = "7360a6779385e4b4f22837ada7537495130916d5"
RUN = 37091483292
ARTIFACT = 11261604898
ARTIFACT_NAME = "SiriModManager-UNSIGNED-37091483292"
EXPECTED = {
    "dist/unsigned/SiriModManager.exe": "2cf93be19c27c4401380cce76b299408745fc301d5f86bbd69531c396ddc2965",
    "dist/unsigned/SiriModManager_Setup.exe": "9a4f34b412f305f31e79c740bbc56d30a53858fecbc901287ad824ed9c221322",
    "LICENSE": "7c721d824c10ba519d35110d770a09f82bc1b53b57f1f8a8e267bae41a90882c",
    "THIRD_PARTY_NOTICES.md": "11d739e412a63bf43e2b30230927c8e15957bbdfd2249cefc1a45ef3d04633b0",
    "third_party/GO-LICENSE.txt": "cfafa52502a751f278b6da9566fa8ec1f1dfaea516283ab56acbb7838f87384a",
}


def gh(*args):
    return subprocess.check_output(["gh", *args], text=True).strip()


def api(path, *args):
    return json.loads(gh("api", f"repos/{REPO}/{path}", *args))


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def prepare_assets(download, output):
    actual = {p.relative_to(download).as_posix() for p in download.rglob("*") if p.is_file()}
    require(actual == set(EXPECTED), "Artifact has unexpected or missing files")
    output.mkdir()
    for name, expected in EXPECTED.items():
        source = download / name
        require(not source.is_symlink() and digest(source) == expected, f"Hash mismatch: {name}")
        shutil.copyfile(source, output / source.name)
    bundle = output / "SiriModManager_v0.8.0_UNSIGNED.zip"
    with zipfile.ZipFile(bundle, "w", compression=zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
        for name in sorted(Path(n).name for n in EXPECTED):
            archive.write(output / name, name)
    sums = "".join(f"{digest(p)}  {p.name}\n" for p in sorted(output.iterdir()))
    (output / "SHA256SUMS.txt").write_text(sums, encoding="utf-8")
    return {p.name: {"size": p.stat().st_size, "digest": "sha256:" + digest(p)} for p in output.iterdir()}


def main():
    require(os.environ.get("GITHUB_REPOSITORY") == REPO, "Wrong repository")
    require(os.environ.get("GITHUB_REF") == "refs/heads/release/publish-v0.8.0", "Wrong branch")
    run = api(f"actions/runs/{RUN}")
    require(run["conclusion"] == "success" and run["head_sha"] == SOURCE, "Unapproved build")
    require(run["path"] == ".github/workflows/build.yml", "Unexpected build workflow")
    artifact = api(f"actions/artifacts/{ARTIFACT}")
    require(not artifact["expired"] and artifact["name"] == ARTIFACT_NAME, "Artifact unavailable")
    require(artifact["workflow_run"]["id"] == RUN and artifact["workflow_run"]["head_sha"] == SOURCE, "Wrong artifact origin")
    releases = api("releases?per_page=100")
    require(not any(r["tag_name"] == TAG for r in releases), "Release already exists; do not overwrite")
    download = Path("release-download")
    output = Path("release-assets")
    require(not download.exists() and not output.exists(), "Output directories already exist")
    gh("run", "download", str(RUN), "--name", ARTIFACT_NAME, "--dir", str(download), "--repo", REPO)
    expected_assets = prepare_assets(download, output)
    refs = api(f"git/matching-refs/tags/{TAG}")
    refs = [r for r in refs if r["ref"] == f"refs/tags/{TAG}"]
    if refs:
        require(len(refs) == 1 and refs[0]["object"]["sha"] == SOURCE, "Existing tag points to different source")
    else:
        api("git/refs", "--method", "POST", "--field", f"ref=refs/tags/{TAG}", "--field", f"sha={SOURCE}")
    gh("release", "create", TAG, "--repo", REPO, "--verify-tag", "--draft", "--title", "Siri ModManager 0.8.0 — unsigniert", "--notes-file", "docs/RELEASE_V0.8.0.md")
    gh("release", "upload", TAG, *[str(p) for p in sorted(output.iterdir())], "--repo", REPO)
    release_id = json.loads(gh("release", "view", TAG, "--repo", REPO, "--json", "databaseId"))["databaseId"]
    release = api(f"releases/{release_id}")
    require(release["draft"], "Expected draft before final publication")
    actual_assets = {a["name"]: a for a in release["assets"]}
    require(set(actual_assets) == set(expected_assets), "Incomplete release upload")
    for name, expected in expected_assets.items():
        uploaded = actual_assets[name]
        require(uploaded["state"] == "uploaded" and uploaded["size"] == expected["size"], f"Incomplete upload: {name}")
        require(uploaded.get("digest") == expected["digest"], f"Uploaded asset digest mismatch: {name}")
    gh("release", "edit", TAG, "--repo", REPO, "--draft=false", "--latest")
    published = api(f"releases/tags/{TAG}")
    require(not published["draft"] and published["published_at"], "Publication not confirmed")
    print(published["html_url"])
    with open(os.environ["GITHUB_STEP_SUMMARY"], "a", encoding="utf-8") as summary:
        summary.write(f"## Unsigned v0.8.0 published\n\n{published['html_url']}\n\nVerified source: `{SOURCE}`\n\nThese binaries are NOT digitally signed.\n")


if __name__ == "__main__":
    main()
