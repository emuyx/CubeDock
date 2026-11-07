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
            "CONTRIBUTING.md",
            "SECURITY.md",
            "LICENSE",
            "NOTICE",
            "THIRD_PARTY_NOTICES.md",
            "config.example.json",
        ):
            target = package / source
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(root / source, target)

        # AGPL-3.0 requires the corresponding source when binaries are
        # distributed. Ship the authentication components, the patched
        # gophertunnel copy and the vendored MIT module with every archive
        # instead of relying on a network offer.
        source_dir = package / "SOURCE"
        source_dir.mkdir()
        (source_dir / "README.md").write_text(
            "Corresponding source for this CubeDock release.\n\n"
            "This directory contains the source of the third-party components\n"
            "embedded in the binary, including the AGPL-3.0 authentication code\n"
            "(vendor-src/g79client and internal/funauth) and the locally patched\n"
            "gophertunnel copy used by limbo. See THIRD_PARTY_NOTICES.md for the\n"
            "full license map.\n",
            encoding="utf-8",
        )
        for source in (
            "vendor-src/g79client",
            "internal/funauth",
            "vendor-src/nemc-tan-lobby-solver",
            "limbo/third_party/gophertunnel",
        ):
            shutil.copytree(
                root / source,
                source_dir / source,
                ignore=shutil.ignore_patterns(
                    "node_modules", "*.exe", "*.zip", "*.mcpack", "*.mcapk"
                ),
            )
        shutil.copyfile(
            root / "vendor-src/g79client/LICENSE",
            source_dir / "internal/funauth/LICENSE",
        )
        for source in (
            "LICENSE",
            "NOTICE",
            "THIRD_PARTY_NOTICES.md",
            "go.mod",
            "go.sum",
            "limbo/go.mod",
            "limbo/go.sum",
            "vendor-src/g79client/go.mod",
            "vendor-src/g79client/go.sum",
            "vendor-src/nemc-tan-lobby-solver/go.mod",
            "vendor-src/nemc-tan-lobby-solver/go.sum",
        ):
            path = root / source
            if not path.exists():
                continue
            target = source_dir / source
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(path, target)

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
