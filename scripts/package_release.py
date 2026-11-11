#!/usr/bin/env python3
"""Package a CubeDock build with documentation and licensing files."""
import argparse
import hashlib
import os
from pathlib import Path
import re
import shutil
import tarfile
import tempfile
import zipfile


def copy_license_tree(root, source, target):
    """Copy only LICENSE/COPYING files and LICENSES directories."""
    base = root / source
    if not base.exists():
        return
    for path in base.rglob("*"):
        if path.is_file() and path.name.upper() in ("LICENSE", "LICENCE", "COPYING"):
            relative = path.relative_to(base)
            destination = target / relative
            destination.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(path, destination)
        elif path.is_dir() and path.name.upper() == "LICENSES":
            for child in path.rglob("*"):
                if not child.is_file():
                    continue
                relative = child.relative_to(base)
                destination = target / relative
                destination.parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(child, destination)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--os", choices=("linux", "windows", "darwin"), required=True)
    parser.add_argument("--arch", choices=("amd64", "arm64"), required=True)
    parser.add_argument("--version", required=True)
    parser.add_argument("--binary", type=Path, required=True)
    parser.add_argument("--output", type=Path, default=Path("dist"))
    args = parser.parse_args()
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9.\-]*", args.version):
        parser.error("version must be a plain version label")

    root = Path(__file__).resolve().parents[1]
    os.chdir(root)
    binary = args.binary.resolve(strict=True)
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=True)
    platform = "macos" if args.os == "darwin" else args.os
    name = f"CubeDock_{args.version}_{platform}_{args.arch}"
    suffix = ".zip" if args.os == "windows" else ".tar.gz"
    archive = output / (name + suffix)

    with tempfile.TemporaryDirectory(prefix=".package-", dir=output) as temporary:
        staging = Path(temporary).resolve()
        if not staging.is_relative_to(output):
            raise RuntimeError("staging directory is outside output")
        package = staging / name
        package.mkdir()
        executable = package / ("CubeDock.exe" if args.os == "windows" else "CubeDock")
        shutil.copyfile(binary, executable)
        executable.chmod(0o755)
        for source in (
            "README.md",
            "SECURITY.md",
            "LICENSE",
            "NOTICE",
            "THIRD_PARTY_NOTICES.md",
            "config.example.json",
        ):
            target = package / source
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(root / source, target)

        # The authentication chain is first-party, so the release no longer
        # ships SOURCE/. Keep the third-party license texts instead.
        license_dir = package / "licenses"
        shutil.copytree(root / "licenses", license_dir)
        vendored = license_dir / "vendored"
        copy_license_tree(root, "vendor-src/nemc-tan-lobby-solver", vendored / "nemc")
        copy_license_tree(root, "limbo/third_party/gophertunnel", vendored / "gophertunnel")

        if args.os == "windows":
            with zipfile.ZipFile(archive, "w", compression=zipfile.ZIP_DEFLATED) as target:
                for path in sorted(package.rglob("*")):
                    if path.is_file():
                        target.write(path, path.relative_to(staging).as_posix())
        else:
            def unix_permissions(info):
                info.mode = 0o755 if info.isdir() or info.name.endswith("/CubeDock") else 0o644
                return info

            with tarfile.open(archive, "w:gz") as target:
                target.add(package, arcname=name, filter=unix_permissions)

    with archive.open("rb") as stream:
        digest = hashlib.file_digest(stream, "sha256").hexdigest()
    archive.with_name(archive.name + ".sha256").write_text(
        f"{digest}  {archive.name}\n", encoding="ascii"
    )
    print(archive.name)


if __name__ == "__main__":
    main()
