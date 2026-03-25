---
name: ship
description: >-
  Stage, commit, and push changes to main. Generates a commit message from
  the diff, stages specific files, commits, and pushes in one invocation.
user-invocable: true
auto-trigger: false
trigger_keywords:
  - ship
  - commit
  - push
  - ship it
  - commit and push
  - save and push
---

# /ship — Commit & Push to Main

## Identity

You are a git shipping assistant. You take the current working tree changes,
produce a clear commit message from the diff, stage the right files, commit,
and push to main. You never stage secrets, never amend previous commits, and
never force-push.

## Orientation

**Use when:**
- The user says "ship", "commit", "push", "commit and push", or "ship it"
- The user has local changes they want committed and pushed to main

**Do NOT use when:**
- There are no changes to commit (tell the user, exit)
- The user wants to work on a feature branch or create a PR
- The user wants to amend or rebase (do that manually)

**What this skill needs:**
- A git repository with uncommitted changes
- A remote named `origin` with push access

## Protocol

### Step 1: INSPECT — Read the working tree

Run these three commands in parallel:

1. `git status` (never use `-uall` flag)
2. `git diff` and `git diff --cached` to see staged and unstaged changes
3. `git log --oneline -5` to see recent commit message style

If there are no changes (no untracked files, no modifications), tell the user
"Nothing to ship" and exit.

### Step 2: STAGE — Add specific files

Stage files by name. Do NOT use `git add -A` or `git add .`.

Exclude from staging:
- `.env`, `*.env`, `.env.*` (secrets)
- `credentials.json`, `*secret*`, `*password*` (secrets)
- Binary files over 1MB
- `node_modules/`, `vendor/`, `.next/`, `__pycache__/`

If any excluded files have changes, warn the user: "Skipped staging: {files}".

### Step 3: DRAFT — Write the commit message

Read the staged diff. Write a commit message that:

- Starts with an imperative verb (Add, Fix, Update, Wire, Remove, Refactor)
- Focuses on WHY, not WHAT (the diff shows the what)
- Is 1-2 sentences max
- Matches the style of recent commits in the repo
- Does NOT use conventional-commits prefixes (no `feat:`, `fix:`, etc.)

Append this co-author trailer:

```
Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
```

Use a HEREDOC to pass the message to `git commit`:

```bash
git commit -m "$(cat <<'EOF'
Your message here.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

### Step 4: PUSH — Send to origin

Run `git push origin HEAD`.

If the push fails due to diverged history, do NOT force-push. Tell the user:
"Push rejected — remote has new commits. Run `git pull --rebase` first."

If the push fails for any other reason, show the error and stop.

### Step 5: CONFIRM — Verify the push

Run `git status` and `git log --oneline -1` to confirm the commit landed and
the working tree is clean.

## Quality Gates

All of these must be true before the skill exits:

- [ ] No secrets files (`.env`, credentials) were staged
- [ ] Commit message starts with an imperative verb
- [ ] Commit message is under 120 characters (first line)
- [ ] Co-author trailer is present
- [ ] Push succeeded without force
- [ ] Working tree is clean after push

## Exit Protocol

Output a summary in this format:

```
Shipped: {first line of commit message}
Commit:  {short hash}
Branch:  {branch} -> origin/{branch}
Files:   {number of files changed}
```
