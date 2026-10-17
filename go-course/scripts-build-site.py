#!/usr/bin/env python3
"""Build go-course/index.html: a single-page course navigator.

Usage (from anywhere):  python3 scripts-build-site.py
Re-run after the course content changes. Your exercise code is never
embedded; the page links to your files and shows the test commands.
"""
import json
import os
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parent
MOD = ROOT / "modules"
SOL = ROOT / "solutions"


def read(p):
    return p.read_text(encoding="utf-8") if p.exists() else ""


def pkg_doc(d):
    """Return the package doc comment of the first non-test .go file in d."""
    for f in sorted(d.glob("*.go")):
        if f.name.endswith("_test.go"):
            continue
        lines, out = f.read_text(encoding="utf-8").splitlines(), []
        for ln in lines:
            if ln.startswith("//"):
                out.append(ln[2:].removeprefix(" "))
            elif ln.startswith("package "):
                return "\n".join(out).strip()
            else:
                out = []
    return ""


def files(d):
    return [
        {"name": str(p.relative_to(d)), "path": str(p)}
        for p in sorted(d.rglob("*"))
        if p.is_file() and "testdata" not in p.parts
    ]


def title_of(md, fallback):
    m = re.search(r"^#\s+(.+)$", md, re.M)
    return m.group(1).strip() if m else fallback


def unit(kind, name, d, rel, test_cmd):
    return {
        "kind": kind,
        "name": name,
        "dir": str(d),
        "doc": pkg_doc(d) if kind != "milestone" else "",
        "readme": read(d / "README.md"),
        "files": files(d),
        "cmd": test_cmd,
        "id": rel,
    }


modules = []
for m in sorted(p for p in MOD.iterdir() if p.is_dir()):
    slug = m.name
    lesson = read(m / "README.md")
    units = []
    ex = m / "exercises"
    if ex.is_dir():
        for e in sorted(p for p in ex.iterdir() if p.is_dir()):
            rel = f"modules/{slug}/exercises/{e.name}"
            units.append(unit("exercise", e.name, e, rel, f"go test ./{rel}/"))
    for ms in sorted(m.glob("milestone*")):
        rel = f"modules/{slug}/{ms.name}"
        units.append(unit("milestone", ms.name, ms, rel, f"go test -race ./{rel}/..."))
    pj = m / "project"
    if pj.is_dir():
        rel = f"modules/{slug}/project"
        units.append(unit("project", "project", pj, rel, f"go test ./{rel}/..."))
    compose = m / "compose.yaml"
    services = []
    if compose.exists():
        in_services = False
        for ln in compose.read_text(encoding="utf-8").splitlines():
            if re.match(r"^\S", ln):
                in_services = ln.startswith("services:")
            elif in_services and (mm := re.match(r"^  ([A-Za-z][\w-]*):\s*$", ln)):
                services.append(mm.group(1))
    modules.append({
        "slug": slug,
        "compose": f"modules/{slug}/compose.yaml" if compose.exists() else "",
        "services": services,
        "title": title_of(lesson, slug),
        "lesson": lesson,
        "units": units,
        "review": read(SOL / slug / "REVIEW.md"),
        "solDir": str(SOL / slug),
        "dir": str(m),
    })

data = {
    "root": str(ROOT),
    "readme": read(ROOT / "README.md"),
    "resources": read(ROOT / "docs" / "RESOURCES.md"),
    "modules": modules,
}

template = read(ROOT / ".authoring" / "site-template.html")
blob = json.dumps(data).replace("</", "<\\/")  # safe inside <script>
out = template.replace("/*__DATA__*/null", blob)
(ROOT / "index.html").write_text(out, encoding="utf-8")
print(f"wrote {ROOT / 'index.html'} ({len(out) // 1024} KiB, {len(modules)} modules)")
