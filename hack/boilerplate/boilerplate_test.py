#!/usr/bin/env python3

# Copyright 2016 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import contextlib
import io
from pathlib import Path
import sys
import tempfile
import unittest
from unittest import mock

# The verifier parses command-line arguments when imported.
with mock.patch.object(sys, "argv", [sys.argv[0]]):
    import boilerplate


class TestBoilerplate(unittest.TestCase):
    def test_boilerplate(self):
        boilerplate_dir = Path(__file__).resolve().parent
        args = boilerplate.parser.parse_args([
            "--rootdir", str(boilerplate_dir / "test"),
            "--boilerplate-dir", str(boilerplate_dir),
        ])
        output = io.StringIO()

        # Fixtures are excluded from normal repository-wide verification.
        with mock.patch.object(boilerplate, "args", args), \
                mock.patch.object(boilerplate, "skipped_dirs", []), \
                contextlib.redirect_stdout(output):
            result = boilerplate.main()

        self.assertEqual(result, 0)
        self.assertEqual(
            sorted(Path(path).name for path in output.getvalue().splitlines()),
            ["fail.go", "fail.py", "fail_2026.go"],
        )

    def test_copyright_headers(self):
        refs = boilerplate.get_refs()
        regexs = boilerplate.get_regexs()
        cases = [("Copyright The Kubernetes Authors.", True)]
        cases.extend(
            (f"Copyright {year} The Kubernetes Authors.", True)
            for year in range(2014, 2026)
        )
        cases.extend([
            ("Copyright 2013 The Kubernetes Authors.", False),
            ("Copyright 2026 The Kubernetes Authors.", False),
            ("Copyright 2099 The Kubernetes Authors.", False),
            ("Copyright YEAR The Kubernetes Authors.", False),
            ("Copyright Other Authors.", False),
        ])
        formats = [
            ("go", "source.go", "//go:build ignore\n\n"),
            ("py", "source.py", "#!/usr/bin/env python3\n\n"),
            ("sh", "source.sh", "#!/usr/bin/env bash\n\n"),
            ("bzl", "source.bzl", ""),
            ("Makefile", "Makefile", ""),
            ("Dockerfile", "Dockerfile", ""),
        ]

        with tempfile.TemporaryDirectory() as directory:
            for extension, filename, prefix in formats:
                header = "\n".join(refs[extension]) + "\n"
                path = Path(directory) / filename
                for notice, expected in cases:
                    with self.subTest(format=extension, notice=notice):
                        path.write_text(
                            prefix + header.replace(
                                "Copyright The Kubernetes Authors.", notice
                            ),
                            encoding="utf-8",
                        )
                        self.assertEqual(
                            boilerplate.file_passes(str(path), refs, regexs),
                            expected,
                        )


if __name__ == "__main__":
    unittest.main()
