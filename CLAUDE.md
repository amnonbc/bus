# Project Guidelines

## Dependencies
- Dependencies are not evil — add a package if it meaningfully improves the code.
- That said, don't add a heavy dependency just for a couple of constants or trivial values (e.g. importing a full terminal package only for two escape sequence strings is not worth it).

## Constants and magic numbers
- Replace magic hex constants with named imports where a suitable package exists (e.g. `golang.org/x/sys/unix`).
- If the constant isn't exported by any package (common with Linux ioctl values like `KDSETMODE`, `EVIOCGRAB`), define it locally with a clear name and comment referencing its origin (e.g. `// from linux/kd.h`).

## Git history
- Keep history clean — combine related commits rather than leaving detours (partial implementations followed by reverts).
- Use `git reset HEAD~N` + recommit to squash when the last few commits tell a messy story.
- Always show the user the diff and wait for approval before committing or pushing.

## Code style
- Avoid closure functions (anonymous functions assigned to variables). Prefer inlining short code or extracting named functions.
- Keep functions short and simple.
- Avoid complex logic in a single function.
- Do not combine an action and its error check in a single `if` condition. Call the function first, then check the error separately. e.g. prefer `err := f(); if err != nil {` on two lines over `if err := f(); err != nil {` on one.

## File editing
- Edit files directly without asking for permission first.
- Running `go build` and `go test` is also fine without asking.