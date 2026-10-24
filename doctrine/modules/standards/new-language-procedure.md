# New-Language Procedure

How a language with no section here gets one. It is part of the new-project process: it
starts when the user explicitly chooses a language that has no section
(`modules/new-project/`).

## Who runs it

**Always the XO, with the user, as a short grill.** The user may instead take a
**suggested section** that the XO writes on its own. Either way the result is recorded as
an addition to that user's standards. It does not change this default; it may be offered
upstream as a change to Trireme.

Without the XO, the user can import a section from a file, or continue with only the
language-neutral rules, recorded as a deviation.

## Steps

1. **For each rule in the profile, state its intent** in one line.
2. **Find what enforces it** in the new language: compiler or interpreter warnings, a
   linter, a static analyzer, a coverage tool, a benchmark tool. Prefer tools whose
   source is public and can be forked.
3. **Mark each rule** as checked by the gate, checked by review, or not applicable, with
   the reason for every "not applicable".
4. **Map the test-name pattern** onto whatever the language's test runner requires.
5. **Map the comment syntax** for the `loop: unbounded` marker and for suppressions.
6. **Note what the language does for you** (for example, freeing memory) and what it
   leaves to the code.
7. **Write the section** in the same table shape as the shipped ones, and record it.
