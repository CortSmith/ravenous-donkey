QUALITY AUDIT of one module in the Go course at /home/csmith/Workspace/tools/go-course (Go 1.24, stdlib only). The module already passes mechanical checks. Your job is CONTENT quality and accuracy. Fix the problems you find directly in the files.

Read /home/csmith/Workspace/tools/go-course/CONVENTIONS.md first. Then audit modules/<m>/ and solutions/<m>/:

1. Lesson README (modules/<m>/README.md): it has all 8 required sections in order (Goals, Concepts, Idioms & Best Practices, Common Pitfalls, Exercises, Project, Submitting for Review, Further Reading). Every exercise and the project is listed with the exact working `go test` command; run each command to confirm the path exists. Code snippets must be correct Go. Extract any non-trivial snippet you doubt into a scratch file under /home/csmith/Workspace/tools/go-course/.authoring/audit-<m>/ and `go run`/`go vet` it. No claims about features newer than Go 1.24 presented as usable. If the README is under ~450 lines, deepen the Concepts with more explanation of the WHY and more examples.
2. The project README exists (modules/<m>/project/README.md, or milestone READMEs for 10-capstone). It has a spec, acceptance criteria, the exact test command, and stretch goals.
3. solutions/<m>/REVIEW.md: for EVERY exercise and the project it has (a) a readable solution, (b) an optimized solution with justification, (c) trade-offs, (d) a grading checklist. Add anything missing.
4. BENCHMARK CLAIMS: every number or speedup in README/REVIEW.md must be real. Run the relevant benchmarks (`go test -run=^$ -bench=. -benchmem ./solutions/<m>/...`) and replace invented or wrong numbers with measured ones, noting "measured on go1.24.2 linux/amd64". If no benchmark exists for a claim, either add one (in a solutions *_test.go file that is not mirrored in modules, e.g. fast_test.go) or reword the claim qualitatively.
5. Never change starter/solution test files that are mirrored between modules/ and solutions/ (they must stay byte-identical). Don't touch other modules.

Finally run /home/csmith/Workspace/tools/go-course/scripts-verify.sh <m>. It must print only the README/REVIEW line counts. Any other line means something is broken; fix it.

Final reply (≤100 words): what you changed and the verify-script output.
